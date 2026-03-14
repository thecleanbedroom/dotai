// Package build provides the build pipeline — extraction + synthesis orchestration.
// All dependencies are domain interfaces, injected by cmd/main.go.
package build

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sync/errgroup"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// Agent orchestrates the build pipeline: commit discovery → batching → extraction → synthesis → DB rebuild.
type Agent struct {
	memWriter domain.MemoryWriter
	memReader domain.MemoryReader
	links     domain.LinkStore
	buildMeta domain.BuildMetaStore
	jsonStore domain.JSONStore
	git       domain.GitParser
	llm       domain.LLMCaller
	cfg       *config.Settings
	dataDir   string
}

// NewAgent creates a BuildAgent with domain-interface dependencies.
func NewAgent(
	memWriter domain.MemoryWriter,
	memReader domain.MemoryReader,
	links domain.LinkStore,
	buildMeta domain.BuildMetaStore,
	jsonStore domain.JSONStore,
	git domain.GitParser,
	llm domain.LLMCaller,
	cfg *config.Settings,
	dataDir string,
) *Agent {
	return &Agent{
		memWriter: memWriter,
		memReader: memReader,
		links:     links,
		buildMeta: buildMeta,
		jsonStore: jsonStore,
		git:       git,
		llm:       llm,
		cfg:       cfg,
		dataDir:   dataDir,
	}
}

// Run executes the full build pipeline.
func (a *Agent) Run(ctx context.Context, limit int, synthesis bool) error {
	// 1. Discover unprocessed commits
	allHashes, err := a.git.GetAllHashes(0)
	if err != nil {
		return fmt.Errorf("get hashes: %w", err)
	}

	processed, err := a.jsonStore.ReadProcessed(a.dataDir)
	if err != nil {
		return fmt.Errorf("read processed: %w", err)
	}

	var unprocessed []string
	for _, h := range allHashes {
		if !processed[h] {
			unprocessed = append(unprocessed, h)
		}
	}

	if limit > 0 && len(unprocessed) > limit {
		unprocessed = unprocessed[:limit]
	}

	fmt.Fprintf(os.Stderr, "  found %d unprocessed commits (of %d total)\n", len(unprocessed), len(allHashes))

	// 2. Run extraction if there are new commits
	if len(unprocessed) > 0 {
		if err := a.extract(ctx, unprocessed); err != nil {
			return fmt.Errorf("extraction: %w", err)
		}
	}

	// 3. Run synthesis if requested
	if synthesis {
		fmt.Fprintln(os.Stderr, "  running synthesis pass...")
		if err := a.runSynthesis(ctx); err != nil {
			return fmt.Errorf("synthesis: %w", err)
		}
	}

	// 4. Record build metadata
	memCount, _ := a.memReader.Count(true)
	a.buildMeta.Record(&domain.BuildMetaEntry{
		BuildType:   "incremental",
		CommitCount: len(unprocessed),
		MemoryCount: memCount,
	})

	return nil
}

func (a *Agent) extract(ctx context.Context, hashes []string) error {
	// Get commits
	commits, err := a.git.GetCommitsByHashes(hashes)
	if err != nil {
		return fmt.Errorf("get commits: %w", err)
	}

	// Apply path filter
	filter := NewPathFilter(a.cfg.FilterIgnorePaths())
	commits = filter.FilterCommits(commits)
	if len(commits) == 0 {
		fmt.Fprintln(os.Stderr, "  all commits filtered out by path filter")
		// Mark as processed
		processMap := make(map[string]bool, len(hashes))
		for _, h := range hashes {
			processMap[h] = true
		}
		return a.jsonStore.AddProcessed(processMap, a.dataDir)
	}

	// Batch commits
	planner := NewBatchPlanner(a.cfg)
	batches := planner.Plan(commits)
	fmt.Fprintf(os.Stderr, "  planned %d batches from %d commits\n", len(batches), len(commits))

	// Extract in parallel with errgroup
	g, gctx := errgroup.WithContext(ctx)
	rpm := 10 // default concurrent extraction limit
	g.SetLimit(rpm)

	for _, batch := range batches {
		batch := batch // capture
		g.Go(func() error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}
			return a.extractBatch(batch)
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Mark all as processed
	processMap := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		processMap[h] = true
	}
	return a.jsonStore.AddProcessed(processMap, a.dataDir)
}

func (a *Agent) extractBatch(commits []*domain.ParsedCommit) error {
	// Build prompt
	prompt := FormatCommitsForExtraction(commits)

	result, err := CallWithRetries(a.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.ExtractionSystemPrompt()},
				{Role: "user", Content: prompt},
			},
			domain.ChatOpts{
				ResponseSchema: config.ExtractionSchema(),
				Label:          "extraction",
			},
		)
	}, nil)
	if err != nil {
		return fmt.Errorf("extraction LLM call: %w", err)
	}

	// Parse and validate
	memories, err := ValidateExtraction(result)
	if err != nil {
		return fmt.Errorf("validate extraction: %w", err)
	}

	// Score and save
	factory := NewMemoryFactory()
	for _, m := range memories {
		scored := factory.Score(m)
		if err := a.jsonStore.Write(scored, a.dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to write memory %s: %v\n", scored.ID, err)
		}
	}

	return nil
}

func (a *Agent) runSynthesis(ctx context.Context) error {
	synth := NewSynthesisAgent(a.llm, a.jsonStore, a.dataDir)
	return synth.Run(ctx)
}

// FormatCommitsForExtraction formats commits as a user prompt for the LLM.
func FormatCommitsForExtraction(commits []*domain.ParsedCommit) string {
	var result string
	for _, c := range commits {
		result += fmt.Sprintf("## Commit %s\n", c.Hash[:8])
		result += fmt.Sprintf("Author: %s\nDate: %s\n", c.Author, c.Date)
		result += fmt.Sprintf("Message: %s\n", c.Message)
		if c.Body != "" {
			result += fmt.Sprintf("Body: %s\n", c.Body)
		}
		if len(c.Trailers) > 0 {
			result += "Trailers:\n"
			for k, v := range c.Trailers {
				result += fmt.Sprintf("  %s: %s\n", k, v)
			}
		}
		if c.Diff != "" {
			result += fmt.Sprintf("\n```diff\n%s\n```\n", c.Diff)
		}
		result += "\n---\n\n"
	}
	return result
}
