package build

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// Agent orchestrates the build pipeline: extract → synthesize → persist.
type Agent struct {
	memWriter domain.MemoryWriter
	memReader domain.MemoryReader
	links     domain.LinkStore
	buildMeta domain.BuildMetaStore
	processed domain.ProcessedTracker
	jsonStore domain.JSONStore
	git       domain.GitParser
	llm       domain.LLMCaller
	cfg       *config.Settings
}

// NewAgent creates a BuildAgent with domain-interface dependencies.
func NewAgent(
	memWriter domain.MemoryWriter,
	memReader domain.MemoryReader,
	links domain.LinkStore,
	buildMeta domain.BuildMetaStore,
	processed domain.ProcessedTracker,
	jsonStore domain.JSONStore,
	git domain.GitParser,
	llm domain.LLMCaller,
	cfg *config.Settings,
) *Agent {
	return &Agent{
		memWriter: memWriter,
		memReader: memReader,
		links:     links,
		buildMeta: buildMeta,
		processed: processed,
		jsonStore: jsonStore,
		git:       git,
		llm:       llm,
		cfg:       cfg,
	}
}

// Run executes the full build pipeline.
func (a *Agent) Run(ctx context.Context, limit int, synthesis bool) error {
	// 1. Discover unprocessed commits
	allHashes, err := a.git.GetAllHashes(0)
	if err != nil {
		return fmt.Errorf("get hashes: %w", err)
	}

	// Build hash lookup set
	hashSet := make(map[string]bool, len(allHashes))
	for _, h := range allHashes {
		hashSet[h] = true
	}

	// 1b. Prune orphaned memories (source_commits no longer in git history)
	pruned, err := a.pruneOrphans(hashSet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: orphan pruning failed: %v\n", err)
	} else if pruned > 0 {
		fmt.Fprintf(os.Stderr, "  pruned %d orphaned memory files\n", pruned)
	}

	processed, err := a.processed.ReadProcessed()
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
	var newMemoryIDs []string
	if len(unprocessed) > 0 {
		ids, err := a.extract(ctx, unprocessed)
		if err != nil {
			return fmt.Errorf("extraction: %w", err)
		}
		newMemoryIDs = ids
	}

	// 3. Run incremental synthesis on NEW memories (automatic)
	if len(newMemoryIDs) > 0 {
		fmt.Fprintf(os.Stderr, "  pass 2: synthesizing %d new memories...\n", len(newMemoryIDs))
		synth := NewSynthesisAgent(a.llm, a.jsonStore)
		if err := synth.RunIncremental(ctx, newMemoryIDs); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: synthesis failed: %v\n", err)
			// Non-fatal — memories are already saved from extraction
		}
	}

	// 4. Full re-synthesis if --synthesis flag
	if synthesis {
		fmt.Fprintln(os.Stderr, "  running full re-synthesis pass...")
		synth := NewSynthesisAgent(a.llm, a.jsonStore)
		if err := synth.RunFull(ctx); err != nil {
			return fmt.Errorf("synthesis: %w", err)
		}
	}

	// 5. Record build metadata
	memCount, err := a.memReader.Count(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: count memories: %v\n", err)
	}
	a.buildMeta.Record(&domain.BuildMetaEntry{
		BuildType:   "incremental",
		CommitCount: len(unprocessed),
		MemoryCount: memCount,
	})

	return nil
}

// extract processes unprocessed commit hashes and returns new memory IDs.
func (a *Agent) extract(ctx context.Context, hashes []string) ([]string, error) {
	// Get commits
	commits, err := a.git.GetCommitsByHashes(hashes)
	if err != nil {
		return nil, fmt.Errorf("get commits: %w", err)
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
		if err := a.processed.AddProcessed(processMap); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to mark commits as processed: %v\n", err)
		}
		return nil, nil
	}

	// Batch commits
	planner := NewBatchPlanner(a.cfg)
	batches := planner.Plan(commits)
	fmt.Fprintf(os.Stderr, "  planned %d batches from %d commits\n", len(batches), len(commits))

	// Extract in parallel with errgroup
	g, gctx := errgroup.WithContext(ctx)
	maxConcurrent := a.cfg.ExtractConcurrency()
	g.SetLimit(maxConcurrent)

	var mu sync.Mutex
	var allNewIDs []string
	processedHashes := make(map[string]bool) // track only successfully extracted commits

	for i, batch := range batches {
		batchIdx := i + 1
		batchTotal := len(batches)
		g.Go(func() error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}
			fmt.Fprintf(os.Stderr, "  batch %d/%d (%d commits)...\n", batchIdx, batchTotal, len(batch))
			ids, err := a.extractBatch(gctx, batch)
			if err != nil {
				return err
			}
			mu.Lock()
			allNewIDs = append(allNewIDs, ids...)
			for _, c := range batch {
				processedHashes[c.Hash] = true
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		// Mark only successfully processed commits — failed batches can be retried
		if len(processedHashes) > 0 {
			if markErr := a.processed.AddProcessed(processedHashes); markErr != nil {
				fmt.Fprintf(os.Stderr, "  warning: failed to mark commits as processed: %v\n", markErr)
			}
		}
		return allNewIDs, err
	}

	// All batches succeeded — mark all hashes as processed
	processMap := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		processMap[h] = true
	}
	if err := a.processed.AddProcessed(processMap); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: failed to mark commits as processed: %v\n", err)
	}
	return allNewIDs, nil
}

// extractBatch extracts memories from a batch and returns new memory IDs.
func (a *Agent) extractBatch(ctx context.Context, commits []*domain.ParsedCommit) ([]string, error) {
	// Build prompt
	prompt := FormatCommitsForExtraction(commits)

	result, err := CallWithRetries(ctx, a.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.ExtractionSystemPrompt()},
				{Role: "user", Content: prompt},
			},
			domain.ChatOpts{
				Ctx:            ctx,
				ResponseSchema: config.ExtractionSchema(),
				Label:          "extraction",
			},
		)
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("extraction LLM call: %w", err)
	}

	// Parse and validate
	memories, err := ValidateExtraction(result)
	if err != nil {
		return nil, fmt.Errorf("validate extraction: %w", err)
	}

	// Score and save
	factory := NewMemoryFactory()
	var newIDs []string
	for _, m := range memories {
		scored := factory.Score(m)

		if err := a.jsonStore.Write(scored); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to write memory %s: %v\n", scored.ID, err)
			continue
		}
		newIDs = append(newIDs, scored.ID)
	}

	fmt.Fprintf(os.Stderr, "    → %d memories extracted\n", len(memories))
	return newIDs, nil
}

// FormatCommitsForExtraction formats commits as a user prompt for the LLM.
func FormatCommitsForExtraction(commits []*domain.ParsedCommit) string {
	var sb strings.Builder
	for _, c := range commits {
		fmt.Fprintf(&sb, "## Commit %s\n", c.Hash)
		fmt.Fprintf(&sb, "Author: %s\nDate: %s\n", c.Author, c.Date)
		fmt.Fprintf(&sb, "Message: %s\n", c.Message)
		if c.Body != "" {
			fmt.Fprintf(&sb, "Body: %s\n", c.Body)
		}
		if len(c.Files) > 0 {
			fmt.Fprintf(&sb, "Files: %v\n", c.Files)
		}
		if len(c.Trailers) > 0 {
			sb.WriteString("Trailers:\n")
			for k, v := range c.Trailers {
				fmt.Fprintf(&sb, "  %s: %s\n", k, v)
			}
		}
		if c.Diff != "" {
			fmt.Fprintf(&sb, "\n```diff\n%s\n```\n", c.Diff)
		}
		sb.WriteString("\n---\n\n")
	}
	return sb.String()
}

// pruneOrphans removes memory files whose source_commits are all absent from
// the current git history (e.g., after rebases or squashes).
func (a *Agent) pruneOrphans(gitHashes map[string]bool) (int, error) {
	memories, err := a.jsonStore.ReadAll()
	if err != nil {
		return 0, err
	}

	pruned := 0
	for _, m := range memories {
		if len(m.SourceCommits) == 0 {
			continue // no source commits → keep
		}

		// Check if ANY source_commit exists in git
		found := false
		for _, h := range m.SourceCommits {
			if gitHashes[h] {
				found = true
				break
			}
		}

		if !found {
			if ok, err := a.jsonStore.Delete(m.ID); err == nil && ok {
				pruned++
				fmt.Fprintf(os.Stderr, "    pruned orphan %s (%s)\n", m.ID[:min(8, len(m.ID))], m.Summary[:min(60, len(m.Summary))])
			}
		}
	}

	return pruned, nil
}

