package llm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/llm"
)

func TestOpenRouter_ListSuitableModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "model/large", "name": "Large Model",
					"context_length": 200000,
					"top_provider":         map[string]any{"max_completion_tokens": 65536},
					"supported_parameters": []string{"temperature", "response_format", "max_tokens"},
					"pricing":              map[string]any{"prompt": "0", "completion": "0"},
				},
				{
					"id": "model/small", "name": "Small Model",
					"context_length": 32000,
					"top_provider":         map[string]any{"max_completion_tokens": 32768},
					"supported_parameters": []string{"temperature", "response_format", "max_tokens"},
					"pricing":              map[string]any{"prompt": "0.001", "completion": "0.002"},
				},
				{
					"id": "model/tiny", "name": "Tiny Model",
					"context_length": 8000,
					"top_provider":         map[string]any{"max_completion_tokens": 32768},
					"supported_parameters": []string{"temperature", "response_format"},
					"pricing":              map[string]any{"prompt": "0", "completion": "0"},
				},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	models, err := or.ListSuitableModels(16000)
	if err != nil {
		t.Fatalf("ListSuitableModels: %v", err)
	}

	for _, m := range models {
		if m.ContextLength < 16000 {
			t.Errorf("model %s has context %d, below minimum 16000", m.ID, m.ContextLength)
		}
	}
}

func TestOpenRouter_ListSuitableModels_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	models, err := or.ListSuitableModels(16000)
	if err != nil {
		t.Fatalf("ListSuitableModels: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestOpenRouter_ListSuitableModels_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	_, err := or.ListSuitableModels(16000)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestOpenRouter_AutoSelectExtractionModel_PrefersFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "free/large", "name": "Free Large",
					"context_length": 200000,
					"top_provider":          map[string]any{"max_completion_tokens": 65536},
					"supported_parameters":  []string{"temperature", "response_format", "max_tokens"},
					"pricing":               map[string]any{"prompt": "0", "completion": "0"},
				},
				{
					"id": "paid/large", "name": "Paid Large",
					"context_length": 200000,
					"top_provider":          map[string]any{"max_completion_tokens": 65536},
					"supported_parameters":  []string{"temperature", "response_format", "max_tokens"},
					"pricing":               map[string]any{"prompt": "0.001", "completion": "0.002"},
				},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	model, err := or.AutoSelectExtractionModel(100000)
	if err != nil {
		t.Fatalf("AutoSelectExtractionModel: %v", err)
	}
	if !model.IsFree {
		t.Errorf("expected free model, got %s (IsFree=%v)", model.ID, model.IsFree)
	}
}

func TestOpenRouter_AutoSelectExtractionModel_NoSuitable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "tiny/model", "name": "Tiny",
					"context_length": 8000,
					"top_provider": map[string]any{"max_completion_tokens": 2048},
					"pricing": map[string]any{"prompt": "0", "completion": "0"},
				},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	_, err := or.AutoSelectExtractionModel(100000)
	if err == nil {
		t.Error("expected error when no models meet context requirement")
	}
}

func TestOpenRouter_EstimateCost_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id": "free/model", "name": "Free",
				"context_length": 128000,
				"pricing": map[string]any{"prompt": "0", "completion": "0"},
			},
		})
	}))
	defer srv.Close()

	or := llm.NewOpenRouter(srv.URL, "test-key")
	cost, err := or.EstimateCost("free/model", 10000, 5000)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if cost != 0 {
		t.Errorf("expected cost 0 for free model, got %f", cost)
	}
}


