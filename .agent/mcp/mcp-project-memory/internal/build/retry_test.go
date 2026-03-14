package build_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/build"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/llm"
)

// mockLLM creates an httptest server returning a canned extraction response.
func mockLLMServer(t *testing.T, response string) (*httptest.Server, *llm.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": response}},
			},
		})
	}))
	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	return srv, client
}

func TestCallWithRetries_Success(t *testing.T) {
	srv, client := mockLLMServer(t, `{"memories":[]}`)
	defer srv.Close()

	result, err := build.CallWithRetries(client, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{{Role: "user", Content: "test"}},
			domain.ChatOpts{Label: "test"},
		)
	}, nil)
	if err != nil {
		t.Fatalf("CallWithRetries: %v", err)
	}
	if result != `{"memories":[]}` {
		t.Errorf("got %q", result)
	}
}

func TestCallWithRetries_FatalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"Bad request"}}`))
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := build.CallWithRetries(client, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{{Role: "user", Content: "test"}},
			domain.ChatOpts{Label: "test"},
		)
	}, nil)
	if err == nil {
		t.Fatal("expected error for fatal 400")
	}
}

func TestFormatCommitsForExtraction(t *testing.T) {
	commits := []*domain.ParsedCommit{
		{
			Hash: "abcdef1234567890abcdef1234567890abcdef12",
			Author: "Test", Date: "2025-01-01",
			Message: "Add feature", Body: "Details here",
			Diff: "+new line", Files: []string{"a.go"},
			Trailers: map[string]string{"Signed-off-by": "Test"},
		},
		{
			Hash: "1234567890abcdef1234567890abcdef12345678",
			Author: "Test2", Date: "2025-01-02",
			Message: "Fix bug",
			Diff: "-old\n+new", Files: []string{"b.go"},
		},
	}

	result := build.FormatCommitsForExtraction(commits)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Should contain commit hashes (first 8 chars)
	if len(result) < 100 {
		t.Errorf("result too short: %d chars", len(result))
	}
}

func TestFormatCommitsForExtraction_Empty(t *testing.T) {
	result := build.FormatCommitsForExtraction(nil)
	if result != "" {
		t.Errorf("expected empty for nil commits, got %q", result)
	}
}
