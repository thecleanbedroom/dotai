// Package main is the entry point for the MCP project-memory server.
// It creates all concrete implementations and injects them into domain interfaces.
//
// Default: stdio transport (for Antigravity auto-launch via mcp_config.json)
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func main() {
	// Parse args: [serve] | build [--synthesis] | reset
	cmd, synthesis := parseArgs()

	// Resolve project root and paths
	projectRoot := storage.ResolveProjectRoot()
	paths := storage.NewPaths(projectRoot)
	if err := paths.EnsureDirs(); err != nil {
		log.Fatalf("ensure directories: %v", err)
	}

	// Load config
	cfg := config.Load(paths.EnvFile)

	switch cmd {
	case "serve":
		runServe(paths, cfg)
	case "build":
		runBuild(paths, cfg, synthesis)
	case "reset":
		runReset(paths)
	case "models":
		runModels(cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
