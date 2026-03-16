package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/config"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/database"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/domain"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/gateway"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/pacing"
	mcpserver "github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/server"
)

// ensureDotEnv copies .env.example to .env if .env doesn't exist.
func ensureDotEnv(paths *config.Paths) {
	if _, err := os.Stat(paths.EnvFile); err == nil {
		return // .env already exists
	}
	src, err := os.Open(paths.EnvExample)
	if err != nil {
		return // no .env.example either
	}
	defer src.Close()

	dst, err := os.Create(paths.EnvFile)
	if err != nil {
		return
	}
	defer dst.Close()
	_, _ = io.Copy(dst, src) // best-effort: .env bootstrap is non-critical
}

// loadDotEnv reads a .env file and sets env vars that aren't already set.
func loadDotEnv(paths *config.Paths) {
	f, err := os.Open(paths.EnvFile)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip surrounding quotes (single or double) — common .env convention
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		// Don't override vars already set in the environment
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

// processExecutor implements gateway.Executor using os/exec.
type processExecutor struct {
	logger *slog.Logger
}

func (e *processExecutor) Run(ctx context.Context, args []string, cwd string, stdin string) (stdout, stderr string, exitCode int, err error) {
	if len(args) == 0 {
		return "", "", 1, fmt.Errorf("no command args")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(stdin)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	execErr := cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
			err = execErr
		}
	}
	return
}

func main() {
	// Resolve project root and paths
	paths := config.NewPaths(config.ResolveProjectRoot())
	if err := paths.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: create data dirs: %v\n", err)
	}

	// Ensure .env exists (copy from .env.example if needed) and load it
	ensureDotEnv(paths)
	loadDotEnv(paths)

	dbPath := flag.String("db", "", "SQLite database path override")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Default()

	// Lock all dispatched agents to the project root — never os.Getwd().
	cfg.ProjectRoot = paths.ProjectRoot

	// Resolve db path: flag > env > paths default
	cfg.DBPath = paths.DBFile
	if *dbPath != "" {
		cfg.DBPath = *dbPath
	} else if envDB := os.Getenv("GATEWAY_DB_PATH"); envDB != "" {
		cfg.DBPath = envDB
	}

	registry := domain.NewModelRegistry(cfg.Models)

	store, err := database.NewStore(cfg, cfg.DBPath, logger)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Seed pacing
	ctx := context.Background()
	if err := store.SeedPacing(ctx, registry, cfg); err != nil {
		logger.Error("failed to seed pacing", "error", err)
		os.Exit(1)
	}

	// Cleanup on startup
	if err := store.CleanStalePIDs(ctx); err != nil {
		logger.Warn("clean stale PIDs", "error", err)
	}
	if err := store.CleanupOldRequests(ctx); err != nil {
		logger.Warn("cleanup old requests", "error", err)
	}

	pacer := pacing.NewManager(store, cfg, registry)
	executor := &processExecutor{logger: logger}
	gw := gateway.NewGateway(store, pacer, executor, cfg, registry, logger)

	srv := mcpserver.New(gw, logger)

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.StartStdio(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
