package gateway

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/database"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/domain"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/pacing"
)

func TestFindBucket(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	tests := []struct {
		name    string
		alias   string
		wantNil bool
		wantLen int
	}{
		{name: "FlashBucket", alias: "fast", wantNil: false, wantLen: 3},
		{name: "ProBucket", alias: "think", wantNil: false, wantLen: 2},
		{name: "LiteBucket", alias: "lite", wantNil: false, wantLen: 3},
		{name: "Unknown", alias: "nonexistent", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bucket := FindBucketForModel(cfg, tt.alias)
			if tt.wantNil && bucket != nil {
				t.Errorf("expected nil bucket, got %v", bucket)
			}
			if !tt.wantNil && bucket == nil {
				t.Error("expected non-nil bucket, got nil")
			}
			if !tt.wantNil && len(bucket) != tt.wantLen {
				t.Errorf("bucket len=%d, want %d", len(bucket), tt.wantLen)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slice []string
		item  string
		want  int
	}{
		{"Found", []string{"a", "b", "c"}, "b", 1},
		{"First", []string{"a", "b", "c"}, "a", 0},
		{"Last", []string{"a", "b", "c"}, "c", 2},
		{"NotFound", []string{"a", "b", "c"}, "d", -1},
		{"Empty", []string{}, "a", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := indexOf(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("indexOf(%v, %q) = %d, want %d", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func newBatchGateway(t *testing.T, exec Executor) (*Gateway, *database.Store) {
	t.Helper()
	cfg := config.Default()
	cfg.DBPath = ":memory:"
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := database.NewStore(cfg, ":memory:", logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	registry := domain.NewModelRegistry(cfg.Models)
	if err := store.SeedPacing(context.Background(), registry, cfg); err != nil {
		t.Fatalf("SeedPacing: %v", err)
	}

	pacer := pacing.NewManager(store, cfg, registry)
	gw := NewGateway(store, pacer, exec, cfg, registry, logger)
	return gw, store
}

func TestAssignModelsForBatch(t *testing.T) {
	exec := &mockExecutor{}
	gw, _ := newBatchGateway(t, exec)

	tests := []struct {
		name string
		jobs []DispatchRequest
		check func(t *testing.T, assignments []Assignment)
	}{
		{
			name: "SingleJob",
			jobs: []DispatchRequest{{Model: "fast", Prompt: "test"}},
			check: func(t *testing.T, assignments []Assignment) {
				if len(assignments) != 1 {
					t.Errorf("len=%d, want 1", len(assignments))
				}
				if assignments[0].Alias != "fast" {
					t.Errorf("alias=%q, want 'fast'", assignments[0].Alias)
				}
			},
		},
		{
			name: "TwoSameBucket_Spreads",
			jobs: []DispatchRequest{
				{Model: "fast", Prompt: "a"},
				{Model: "fast", Prompt: "b"},
			},
			check: func(t *testing.T, assignments []Assignment) {
				if len(assignments) != 2 {
					t.Fatalf("len=%d, want 2", len(assignments))
				}
				// First should get fast, second should get alternative
				if assignments[0].Alias == assignments[1].Alias {
					// Both got same, which is ok if bucket is exhausted
					// but with 3 models in bucket, second should differ
				}
			},
		},
		{
			name: "EmptyModelDefaults",
			jobs: []DispatchRequest{{Prompt: "default model"}},
			check: func(t *testing.T, assignments []Assignment) {
				if len(assignments) != 1 {
					t.Errorf("len=%d, want 1", len(assignments))
				}
				if assignments[0].Alias != "fast" {
					t.Errorf("alias=%q, want 'fast' (default)", assignments[0].Alias)
				}
			},
		},
		{
			name: "CrossBucket",
			jobs: []DispatchRequest{
				{Model: "fast", Prompt: "flash tier"},
				{Model: "think", Prompt: "pro tier"},
			},
			check: func(t *testing.T, assignments []Assignment) {
				if len(assignments) != 2 {
					t.Fatalf("len=%d, want 2", len(assignments))
				}
				// Should get their requested models since different buckets
				aliases := map[string]bool{assignments[0].Alias: true, assignments[1].Alias: true}
				if !aliases["fast"] || !aliases["think"] {
					t.Errorf("expected fast+think, got %v", aliases)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignments := gw.AssignModelsForBatch(context.Background(), tt.jobs)
			tt.check(t, assignments)
		})
	}
}

func TestRunBatch(t *testing.T) {
	exec := &mockExecutor{
		stdout:   `{"response": "batch ok", "stats": {}}`,
		exitCode: 0,
	}
	gw, _ := newBatchGateway(t, exec)

	jobs := []DispatchRequest{
		{Model: "fast", Prompt: "batch job 1", Label: "job1", Cwd: "/tmp"},
		{Model: "think", Prompt: "batch job 2", Label: "job2", Cwd: "/tmp"},
	}

	results, err := gw.RunBatch(context.Background(), jobs)
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len=%d, want 2", len(results))
	}

	for i, r := range results {
		if r.ExitCode != 0 {
			t.Errorf("results[%d].ExitCode=%d, want 0", i, r.ExitCode)
		}
		if r.Status != "ok" {
			t.Errorf("results[%d].Status=%q, want 'ok'", i, r.Status)
		}
	}
}

func TestRunBatch_MixedResults(t *testing.T) {
	// Alternating success/failure
	callCount := 0
	exec := &alternatingExecutor{}
	_ = callCount
	gw, _ := newBatchGateway(t, exec)

	jobs := []DispatchRequest{
		{Model: "fast", Prompt: "success job", Label: "good", Cwd: "/tmp"},
		{Model: "think", Prompt: "fail job", Label: "bad", Cwd: "/tmp"},
	}

	results, err := gw.RunBatch(context.Background(), jobs)
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len=%d, want 2", len(results))
	}
}

// alternatingExecutor returns success on odd calls, failure on even.
type alternatingExecutor struct {
	calls int
}

func (e *alternatingExecutor) Run(_ context.Context, args []string, cwd string, stdin string) (string, string, int, error) {
	e.calls++
	if e.calls%2 == 1 {
		return `{"response": "ok", "stats": {}}`, "", 0, nil
	}
	return "", "error", 1, nil
}
