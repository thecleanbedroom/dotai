package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/inspector"
	"github.com/dotai/mcp-project-memory/internal/server"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func runServe(paths *storage.Paths, cfg *config.Settings) {
	// Create concrete implementations
	db, err := storage.NewDatabase(paths.DBFile)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	linkStore := storage.NewLinkStore(db)
	memStore := storage.NewMemoryStore(db, linkStore)
	buildMeta := storage.NewBuildMetaStore(db)
	jsonStore := storage.NewJSONStore(paths)

	// Raw querier adapter for inspector
	rawDB := &sqliteRawQuerier{db: db}

	// Inspector
	insp := inspector.New(memStore, linkStore, buildMeta, db, rawDB)

	// McpServer (adapter)
	rebuilder := storage.NewRebuilder(db, memStore, linkStore, jsonStore)
	mcpSrv := &server.McpServer{
		MemReader: memStore,
		MemWriter: memStore,
		Searcher:  memStore,
		Links:     linkStore,
		Builds:    buildMeta,
		Inspector: insp,
		DB:        db,
		JSONStore: jsonStore,
		Rebuilder: rebuilder,
	}

	// Initial freshness check
	rebuilt, err := mcpSrv.EnsureFresh()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: freshness check failed: %v\n", err)
	}
	if rebuilt {
		fmt.Fprintln(os.Stderr, "DB rebuilt from JSON on startup")
	}

	// Initialize MCP server (tools + prompts)
	mcpSrv.Init(memStore.ListAll)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Stdio mode: default for Antigravity auto-launch
	if err := mcpSrv.StartStdio(ctx); err != nil {
		log.Fatalf("stdio: %v", err)
	}
}
