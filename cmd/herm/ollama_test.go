package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- fetchOllamaModels tests ---

func TestFetchOllamaModelsEmptyURL(t *testing.T) {
	models := fetchOllamaModels("")
	if models != nil {
		t.Errorf("expected nil for empty URL, got %d models", len(models))
	}
}

func TestFetchOllamaModelsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"name": "model-a"},
					{"name": "model-b"},
				},
			})
		case "/api/show":
			json.NewEncoder(w).Encode(map[string]any{
				"model_info": map[string]any{
					"general.context_length": 131072,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	models := fetchOllamaModels(srv.URL)
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0].Provider != ProviderOllama {
		t.Errorf("Provider = %q, want %q", models[0].Provider, ProviderOllama)
	}
	if models[0].PromptPrice != 0 || models[0].CompletionPrice != 0 {
		t.Error("local Ollama models should have zero price")
	}
	if models[0].ContextWindow != 131072 {
		t.Errorf("ContextWindow = %d, want 131072 (from /api/show)", models[0].ContextWindow)
	}
}

func TestFetchOllamaModelsNoContextLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "some-model"}},
			})
		case "/api/show":
			// model_info without any context_length key
			json.NewEncoder(w).Encode(map[string]any{
				"model_info": map[string]any{
					"general.embedding_length": 4096,
				},
			})
		}
	}))
	defer srv.Close()

	models := fetchOllamaModels(srv.URL)
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].ContextWindow != 0 {
		t.Errorf("ContextWindow = %d, want 0 when /api/show has no context_length", models[0].ContextWindow)
	}
}

func TestFetchOllamaModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	models := fetchOllamaModels(srv.URL)
	if models != nil {
		t.Errorf("expected nil on server error, got %d models", len(models))
	}
}

func TestFetchOllamaModelsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	models := fetchOllamaModels(srv.URL)
	if models != nil {
		t.Errorf("expected nil on invalid JSON, got %d models", len(models))
	}
}

func TestFetchOllamaModelsUnreachable(t *testing.T) {
	models := fetchOllamaModels("http://127.0.0.1:1")
	if models != nil {
		t.Errorf("expected nil for unreachable server, got %d models", len(models))
	}
}

func TestFetchOllamaModelsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
	}))
	defer srv.Close()

	models := fetchOllamaModels(srv.URL)
	if len(models) != 0 {
		t.Errorf("expected 0 models for empty list, got %d", len(models))
	}
}
