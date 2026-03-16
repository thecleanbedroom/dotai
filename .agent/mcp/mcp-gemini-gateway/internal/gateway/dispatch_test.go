package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/config"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/database"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/domain"
	"github.com/thecleanbedroom/dotai/mcp-gemini-gateway/internal/pacing"
)

// mockExecutor implements Executor for testing dispatch flow.
type mockExecutor struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
	calls    int
}

func (m *mockExecutor) Run(_ context.Context, args []string, cwd string, stdin string) (string, string, int, error) {
	m.calls++
	return m.stdout, m.stderr, m.exitCode, m.err
}

// newFullTestGateway creates a Gateway with all dependencies for dispatch testing.
func newFullTestGateway(t *testing.T, exec Executor) (*Gateway, *database.Store) {
	t.Helper()
	cfg := config.Default()
	cfg.DBPath = ":memory:"
	cfg.ProjectRoot = "/tmp"
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

func TestDispatch_Success(t *testing.T) {
	exec := &mockExecutor{
		stdout:   `{"response": "Hello world", "stats": {}}`,
		exitCode: 0,
	}
	gw, _ := newFullTestGateway(t, exec)

	result, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:  "fast",
		Prompt: "say hello",
		Label:  "test-job",
		Cwd:    "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit_code=%d, want 0", result.ExitCode)
	}
	if result.RequestID == 0 {
		t.Error("request_id should be non-zero")
	}
	if exec.calls != 1 {
		t.Errorf("executor calls=%d, want 1", exec.calls)
	}
}

func TestDispatch_UnknownModel(t *testing.T) {
	exec := &mockExecutor{}
	gw, _ := newFullTestGateway(t, exec)

	_, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:  "nonexistent",
		Prompt: "test",
	})
	if err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestDispatch_RateLimit_Retries(t *testing.T) {
	// First call returns rate limit, second succeeds
	customExec := &rateLimitThenSuccessExecutor{
		rateLimitCalls: 1,
	}

	gw, _ := newFullTestGateway(t, customExec)

	result, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:  "fast",
		Prompt: "test retry",
		Label:  "retry-test",
		Cwd:    "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit_code=%d, want 0 (should recover after retry)", result.ExitCode)
	}
	if customExec.calls < 2 {
		t.Errorf("calls=%d, want >= 2 (at least 1 retry)", customExec.calls)
	}
}

// rateLimitThenSuccessExecutor returns rate-limit N times, then success.
type rateLimitThenSuccessExecutor struct {
	rateLimitCalls int
	calls          int
}

func (e *rateLimitThenSuccessExecutor) Run(_ context.Context, args []string, cwd string, stdin string) (string, string, int, error) {
	e.calls++
	if e.calls <= e.rateLimitCalls {
		return "", "429 Too Many Requests", 1, nil
	}
	return `{"response": "ok", "stats": {}}`, "", 0, nil
}

func TestDispatch_Failure(t *testing.T) {
	exec := &mockExecutor{
		stdout:   "",
		stderr:   "command failed",
		exitCode: 1,
	}
	gw, _ := newFullTestGateway(t, exec)

	result, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:  "fast",
		Prompt: "fail test",
		Label:  "fail-job",
		Cwd:    "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code for failure")
	}
}

func TestDispatch_ExecutorError(t *testing.T) {
	exec := &mockExecutor{
		err: fmt.Errorf("connection refused"),
	}
	gw, _ := newFullTestGateway(t, exec)

	result, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:  "fast",
		Prompt: "error test",
		Cwd:    "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit for executor error")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestDispatch_Sandbox(t *testing.T) {
	exec := &mockExecutor{
		stdout:   `{"response": "sandbox result", "stats": {}}`,
		exitCode: 0,
	}
	gw, _ := newFullTestGateway(t, exec)

	result, err := gw.Dispatch(context.Background(), DispatchRequest{
		Model:   "fast",
		Prompt:  "sandbox test",
		Sandbox: true,
		Cwd:     "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit_code=%d, want 0", result.ExitCode)
	}
}

func TestDispatch_QueueFull(t *testing.T) {
	exec := &mockExecutor{
		stdout:   `{"response": "ok", "stats": {}}`,
		exitCode: 0,
	}
	gw, store := newFullTestGateway(t, exec)

	// Fill the queue to capacity
	ctx := context.Background()
	model, _ := gw.registry.Resolve("fast")
	maxQueue := gw.cfg.MaxQueue["fast"]
	for range maxQueue {
		req := &domain.Request{
			Model: model, Status: "waiting", PromptHash: "hash",
			PID: 0, Cwd: "/tmp", CreatedAt: float64(time.Now().Unix()),
		}
		store.InsertRequest(ctx, req)
	}

	result, err := gw.Dispatch(ctx, DispatchRequest{
		Model:  "fast",
		Prompt: "queue full test",
		Cwd:    "/tmp",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.ExitCode != 2 {
		t.Errorf("exit_code=%d, want 2 (queue full)", result.ExitCode)
	}
	if result.Error == "" {
		t.Error("expected queue full error message")
	}
}

// ════════ parseGeminiOutput tests ════════

func TestParseGeminiOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantResp  string
		wantStats bool
	}{
		{
			name:     "ValidJSON",
			input:    `{"response": "Hello!", "stats": {"models": {"gemini-3-flash": {"tokens": {"input": 100, "candidates": 200}, "api": {"totalLatencyMs": 500}}}, "tools": {"totalCalls": 3}}}`,
			wantResp: "Hello!",
			wantStats: true,
		},
		{
			name:     "InvalidJSON",
			input:    "This is not JSON output",
			wantResp: "This is not JSON output",
			wantStats: false,
		},
		{
			name:     "EmptyJSON",
			input:    `{"response": "", "stats": {}}`,
			wantResp: "",
			wantStats: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, stats := parseGeminiOutput(tt.input)
			if resp != tt.wantResp {
				t.Errorf("response=%q, want %q", resp, tt.wantResp)
			}
			if tt.wantStats && len(stats) == 0 {
				t.Error("expected non-empty stats")
			}
			if tt.wantStats {
				if _, ok := stats["tokens_in"]; !ok {
					t.Error("missing tokens_in in stats")
				}
				if _, ok := stats["tool_calls"]; !ok {
					t.Error("missing tool_calls in stats")
				}
			}
		})
	}
}

func TestFindBucketAlternative(t *testing.T) {
	exec := &mockExecutor{}
	gw, _ := newFullTestGateway(t, exec)

	// No running models → returns a bucket peer since nothing is marked unavailable
	alt := gw.findBucketAlternative(context.Background(), "fast")
	// "fast" is in the ["lite", "quick", "fast"] bucket — should get a peer
	validPeers := map[string]bool{"lite": true, "quick": true}
	if !validPeers[alt] {
		t.Errorf("findBucketAlternative('fast') = %q, want one of lite/quick", alt)
	}
}
