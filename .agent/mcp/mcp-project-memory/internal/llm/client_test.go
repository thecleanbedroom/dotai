package llm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/llm"
)

func TestClient_Chat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing bearer token")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong content type")
		}

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "test-model" {
			t.Errorf("model: got %v", req["model"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "Hello from mock"}},
			},
			"usage": map[string]any{
				"prompt_tokens": 10, "completion_tokens": 5,
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	result, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "Hello"}},
		domain.ChatOpts{Label: "test"},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != "Hello from mock" {
		t.Errorf("expected 'Hello from mock', got %q", result)
	}
}

func TestClient_Chat_WithSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		// Should have response_format
		if req["response_format"] == nil {
			t.Error("expected response_format to be set")
		}

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": `{"memories":[]}`}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	result, err := client.Chat(
		[]domain.Message{
			{Role: "system", Content: "Extract"},
			{Role: "user", Content: "commit data"},
		},
		domain.ChatOpts{
			ResponseSchema: json.RawMessage(`{"type":"object"}`),
			Label:          "extraction",
		},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != `{"memories":[]}` {
		t.Errorf("got %q", result)
	}
}

func TestClient_Chat_MaxTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)

		if req["max_tokens"] != float64(500) {
			t.Errorf("max_tokens: got %v", req["max_tokens"])
		}

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{MaxTokens: 500, Label: "test"},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
}

func TestClient_Chat_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"message":"Internal server error"}}`))
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{Label: "test"},
	)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	apiErr, ok := err.(*llm.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsTransient() {
		t.Error("500 should be transient")
	}
}

func TestClient_Chat_RateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"Rate limited"}}`))
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{Label: "test"},
	)
	if err == nil {
		t.Fatal("expected error for 429")
	}
	apiErr, ok := err.(*llm.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if !apiErr.IsRateLimit() {
		t.Error("429 should be rate limit")
	}
	if !apiErr.IsTransient() {
		t.Error("429 should also be transient")
	}
}

func TestClient_Chat_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{Label: "test"},
	)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestClient_Chat_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Model not found", "code": 404},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "test-model", "")
	_, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{Label: "test"},
	)
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestClient_Chat_WithLogging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "logged"}},
			},
		})
	}))
	defer srv.Close()

	logDir := t.TempDir()
	client := llm.NewClient(srv.URL, "test-key", "test-model", logDir)
	result, err := client.Chat(
		[]domain.Message{{Role: "user", Content: "hello"}},
		domain.ChatOpts{Label: "logtest"},
	)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != "logged" {
		t.Errorf("expected 'logged', got %q", result)
	}
}

func TestAPIError_Methods(t *testing.T) {
	tests := []struct {
		code        int
		isRateLimit bool
		isTransient bool
	}{
		{200, false, false},
		{400, false, false},
		{403, false, false},
		{429, true, true},
		{500, false, true},
		{502, false, true},
		{503, false, true},
	}
	for _, tt := range tests {
		e := &llm.APIError{StatusCode: tt.code, Body: "test"}
		if e.IsRateLimit() != tt.isRateLimit {
			t.Errorf("code %d IsRateLimit: got %v", tt.code, e.IsRateLimit())
		}
		if e.IsTransient() != tt.isTransient {
			t.Errorf("code %d IsTransient: got %v", tt.code, e.IsTransient())
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	e := &llm.APIError{StatusCode: 429, Body: "rate limited"}
	msg := e.Error()
	if msg == "" {
		t.Error("Error() should return non-empty string")
	}
}
