package llm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/llm"
)

func TestOpenRouter_GetModelInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":              "test/model",
				"name":            "Test Model",
				"context_length":  128000,
				"top_provider_max_completion_tokens": 4096,
				"pricing": map[string]any{
					"prompt":     "0.001",
					"completion": "0.002",
				},
				"supported_parameters": []string{"temperature", "max_tokens"},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	info, err := or.GetModelInfo("test/model")
	if err != nil {
		t.Fatalf("GetModelInfo: %v", err)
	}
	if info.Name != "Test Model" {
		t.Errorf("name: got %q", info.Name)
	}
	if info.ContextLength != 128000 {
		t.Errorf("context: got %d", info.ContextLength)
	}
}

func TestOpenRouter_GetModelInfo_Cached(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "cached/model",
				"name":           "Cached Model",
				"context_length": 64000,
				"pricing":        map[string]any{"prompt": "0", "completion": "0"},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	or.GetModelInfo("cached/model")
	or.GetModelInfo("cached/model")

	if callCount != 1 {
		t.Errorf("expected 1 API call (cached), got %d", callCount)
	}
}

func TestOpenRouter_GetModelInfo_FreeModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "free/model",
				"name":           "Free Model",
				"context_length": 32000,
				"pricing":        map[string]any{"prompt": "0", "completion": "0"},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	info, _ := or.GetModelInfo("free/model")
	if !info.IsFree {
		t.Error("expected IsFree=true for zero pricing")
	}
}

func TestOpenRouter_GetModelInfo_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	_, err := or.GetModelInfo("bad/model")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestOpenRouter_EstimateCost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "priced/model",
				"name":           "Priced Model",
				"context_length": 128000,
				"pricing":        map[string]any{"prompt": "0.001", "completion": "0.002"},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	cost, err := or.EstimateCost("priced/model", 1000, 500)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	// Cost should be non-negative
	if cost < 0 {
		t.Errorf("expected non-negative cost, got %f", cost)
	}
}

func TestClient_GetModelInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":             "client/model",
				"name":           "Client Model",
				"context_length": 128000,
				"pricing":        map[string]any{"prompt": "0", "completion": "0"},
			},
		})
	}))
	defer srv.Close()

	client := llm.NewClient(srv.URL, "test-key", "client/model", "")
	info, err := client.GetModelInfo()
	if err != nil {
		t.Fatalf("GetModelInfo: %v", err)
	}
	if info.Name != "Client Model" {
		t.Errorf("name: got %q", info.Name)
	}
}


