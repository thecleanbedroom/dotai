package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dotai/mcp-project-memory/internal/build"
	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/git"
	"github.com/dotai/mcp-project-memory/internal/llm"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func runBuild(paths *storage.Paths, cfg *config.Settings, synthesis bool) {
	// Validate API key — prompt if missing, persist to .env
	if cfg.APIKey() == "" {
		fmt.Fprint(os.Stderr, "OPENROUTER_API_KEY not set. Enter API key: ")
		var key string
		fmt.Scanln(&key)
		if key == "" {
			log.Fatal("API key is required for build")
		}
		os.Setenv("OPENROUTER_API_KEY", key)
		persistEnvKey(paths.EnvFile, "OPENROUTER_API_KEY", key)
	}

	// Resolve model selections (auto = best free model)
	router := llm.NewOpenRouter(cfg.APIURL(), cfg.APIKey())
	batchBudget := cfg.BatchingTokenBudget()
	extractMinCtx := batchBudget + config.ExtractionOverheadTokens() + 16384

	extractModel := cfg.ExtractionModel()
	if extractModel == "auto" {
		m, err := router.AutoSelectExtractionModel(extractMinCtx)
		if err != nil {
			log.Fatalf("auto-select extraction model: %v", err)
		}
		extractModel = m.ID
		fmt.Fprintf(os.Stderr, "  extraction model: %s (auto, %dk ctx)\n", m.ID, m.ContextLength/1000)
	} else {
		fmt.Fprintf(os.Stderr, "  extraction model: %s (configured)\n", extractModel)
	}

	fallbackModel := cfg.ExtractionFallbackModel()
	if fallbackModel == "auto" {
		fallbackModel = extractModel // same as primary by default
	}

	synthesisModel := cfg.SynthesisModel()
	if synthesisModel == "auto" {
		m, err := router.AutoSelectReasoningModel(262000, 65536)
		if err != nil {
			// Non-fatal — synthesis may not be requested
			fmt.Fprintf(os.Stderr, "  reasoning model:  (auto, none found: %v)\n", err)
		} else {
			synthesisModel = m.ID
			fmt.Fprintf(os.Stderr, "  reasoning model:  %s (auto, %dk ctx, %dk max out)\n", m.ID, m.ContextLength/1000, m.MaxCompletionTokens/1000)
		}
	} else {
		fmt.Fprintf(os.Stderr, "  reasoning model:  %s (configured)\n", synthesisModel)
	}

	// Open DB
	db, err := storage.NewDatabase(paths.DBFile)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Create stores
	linkStore := storage.NewLinkStore(db)
	memStore := storage.NewMemoryStore(db, linkStore)
	buildMeta := storage.NewBuildMetaStore(db)
	jsonStore := storage.NewJSONStore(paths)

	// Seed processed_commits if empty (one-time migration from JSON)
	processed, err := db.ReadProcessed()
	if err != nil {
		log.Fatalf("read processed: %v", err)
	}
	if len(processed) == 0 {
		memories, err := jsonStore.ReadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: read JSON for migration: %v\n", err)
		}
		commitSet := map[string]bool{}
		for _, m := range memories {
			for _, h := range m.SourceCommits {
				commitSet[h] = true
			}
		}
		if len(commitSet) > 0 {
			if err := db.AddProcessed(commitSet); err != nil {
				log.Fatalf("seed processed: %v", err)
			}
			fmt.Fprintf(os.Stderr, "  seeded %d processed commits from memory files\n", len(commitSet))
		}
	}

	// Detect git repo root (walk up from project dir)
	repoRoot := detectGitRoot(paths.ProjectRoot)
	gitParser := git.NewParser(repoRoot)

	// Create LLM client
	llmClient := llm.NewClient(cfg.APIURL(), cfg.APIKey(), extractModel, paths.LLMLogsDir)

	// Create build agent
	agent := build.NewAgent(memStore, memStore, linkStore, buildMeta, db, jsonStore, gitParser, llmClient, cfg)

	// Run pipeline
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	limit := cfg.BatchingCommitLimit()
	fmt.Fprintf(os.Stderr, "starting build (synthesis=%v, limit=%d)...\n", synthesis, limit)
	if err := agent.Run(ctx, limit, synthesis); err != nil {
		log.Fatalf("build failed: %v", err)
	}

	// Rebuild SQLite from updated JSON
	fmt.Fprintln(os.Stderr, "rebuilding SQLite from JSON...")
	count, err := storage.RebuildDBFromJSON(db, memStore, linkStore, jsonStore, nil)
	if err != nil {
		log.Fatalf("rebuild DB: %v", err)
	}
	fmt.Fprintf(os.Stderr, "build complete: %d memories in DB\n", count)
}
