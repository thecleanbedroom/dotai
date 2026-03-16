package testutil

import (
	"log/slog"
	"os"
	"testing"

	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/config"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/database"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/domain"
)

// NewTestConfig returns a Config with defaults for testing.
func NewTestConfig() *config.Config {
	cfg := config.Default()
	cfg.DBPath = ":memory:"
	return cfg
}

// NewTestRegistry returns a ModelRegistry from test config.
func NewTestRegistry() *domain.ModelRegistry {
	return domain.NewModelRegistry(NewTestConfig().Models)
}

// NewTestStore creates an in-memory database Store for testing.
func NewTestStore(t *testing.T) *database.Store {
	t.Helper()
	cfg := NewTestConfig()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	store, err := database.NewStore(cfg, ":memory:", logger)
	if err != nil {
		t.Fatalf("NewTestStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Seed pacing
	registry := NewTestRegistry()
	if err := store.SeedPacing(t.Context(), registry, cfg); err != nil {
		t.Fatalf("SeedPacing: %v", err)
	}
	return store
}

// InsertRequest inserts a test request with common defaults.
func InsertRequest(t *testing.T, store *database.Store, model, status string, opts ...func(*domain.Request)) int64 {
	t.Helper()
	req := &domain.Request{
		Model:      model,
		Status:     status,
		PromptHash: "testhash",
		PID:        os.Getpid(),
		Cwd:        "/tmp",
		CreatedAt:  domain.NowUnix(),
	}
	for _, opt := range opts {
		opt(req)
	}
	id, err := store.InsertRequest(t.Context(), req)
	if err != nil {
		t.Fatalf("InsertRequest: %v", err)
	}
	return id
}

// WithLabel sets the label on a request.
func WithLabel(label string) func(*domain.Request) {
	return func(r *domain.Request) { r.Label = label }
}

// WithBatchID sets the batch ID on a request.
func WithBatchID(batchID string) func(*domain.Request) {
	return func(r *domain.Request) { r.BatchID = batchID }
}

// WithPID sets the PID on a request.
func WithPID(pid int) func(*domain.Request) {
	return func(r *domain.Request) { r.PID = pid }
}

// WithCreatedAt sets the created_at timestamp.
func WithCreatedAt(ts float64) func(*domain.Request) {
	return func(r *domain.Request) { r.CreatedAt = ts }
}

// WithError sets the error field.
func WithError(errMsg string) func(*domain.Request) {
	return func(r *domain.Request) { r.Error = errMsg }
}

// WithPromptText sets the prompt text field.
func WithPromptText(text string) func(*domain.Request) {
	return func(r *domain.Request) { r.PromptText = text }
}
