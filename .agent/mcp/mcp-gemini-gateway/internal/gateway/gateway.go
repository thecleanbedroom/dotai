package gateway

import (
	"context"
	"log/slog"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/database"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/domain"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/pacing"
)

// Executor abstracts subprocess execution for testability.
type Executor interface {
	Run(ctx context.Context, args []string, cwd string, stdin string) (stdout, stderr string, exitCode int, err error)
}

// Gateway is the central orchestrator — owns dispatch, batch, and commands.
// Dependencies are injected via constructor (DIP).
type Gateway struct {
	store    *database.Store
	pacer    pacing.Pacer
	executor Executor
	cfg      *config.Config
	registry *domain.ModelRegistry
	logger   *slog.Logger
}

// NewGateway creates a Gateway with all dependencies injected.
func NewGateway(
	store *database.Store,
	pacer pacing.Pacer,
	executor Executor,
	cfg *config.Config,
	registry *domain.ModelRegistry,
	logger *slog.Logger,
) *Gateway {
	return &Gateway{
		store:    store,
		pacer:    pacer,
		executor: executor,
		cfg:      cfg,
		registry: registry,
		logger:   logger,
	}
}
