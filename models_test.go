package main

import "testing"

func TestFilterModelsByProvidersSingle(t *testing.T) {
	providers := map[string]bool{ProviderAnthropic: true}
	models := filterModelsByProviders(providers)

	for _, m := range models {
		if m.Provider != ProviderAnthropic {
			t.Errorf("expected only anthropic models, got provider %q", m.Provider)
		}
	}
	if len(models) == 0 {
		t.Error("expected at least one anthropic model")
	}
}

func TestFilterModelsByProvidersMultiple(t *testing.T) {
	providers := map[string]bool{ProviderGrok: true, ProviderOpenAI: true}
	models := filterModelsByProviders(providers)

	for _, m := range models {
		if m.Provider != ProviderGrok && m.Provider != ProviderOpenAI {
			t.Errorf("unexpected provider %q", m.Provider)
		}
	}
	if len(models) == 0 {
		t.Error("expected models for grok and openai")
	}
}

func TestFilterModelsByProvidersEmpty(t *testing.T) {
	models := filterModelsByProviders(map[string]bool{})
	if len(models) != 0 {
		t.Errorf("expected no models, got %d", len(models))
	}
}

func TestFindModelByIDFound(t *testing.T) {
	m := findModelByID("gpt-4o")
	if m == nil {
		t.Fatal("expected to find gpt-4o")
	}
	if m.Provider != ProviderOpenAI {
		t.Errorf("provider = %q, want openai", m.Provider)
	}
}

func TestFindModelByIDNotFound(t *testing.T) {
	m := findModelByID("nonexistent-model")
	if m != nil {
		t.Errorf("expected nil for nonexistent model, got %+v", m)
	}
}

func TestConfiguredProvidersNone(t *testing.T) {
	cfg := Config{}
	p := cfg.configuredProviders()
	if len(p) != 0 {
		t.Errorf("expected no providers, got %v", p)
	}
}

func TestConfiguredProvidersSome(t *testing.T) {
	cfg := Config{AnthropicAPIKey: "sk-ant-123", OpenAIAPIKey: "sk-openai-456"}
	p := cfg.configuredProviders()

	if !p[ProviderAnthropic] {
		t.Error("expected anthropic to be configured")
	}
	if p[ProviderGrok] {
		t.Error("expected grok to NOT be configured")
	}
	if !p[ProviderOpenAI] {
		t.Error("expected openai to be configured")
	}
}

func TestConfiguredProvidersAll(t *testing.T) {
	cfg := Config{
		AnthropicAPIKey: "key1",
		GrokAPIKey:      "key2",
		OpenAIAPIKey:    "key3",
	}
	p := cfg.configuredProviders()
	if len(p) != 3 {
		t.Errorf("expected 3 providers, got %d", len(p))
	}
}

func TestAvailableModelsFilters(t *testing.T) {
	cfg := Config{GrokAPIKey: "xai-key"}
	models := cfg.availableModels()

	for _, m := range models {
		if m.Provider != ProviderGrok {
			t.Errorf("expected only grok models, got provider %q", m.Provider)
		}
	}
	if len(models) == 0 {
		t.Error("expected at least one grok model")
	}
}

func TestResolveActiveModelValid(t *testing.T) {
	cfg := Config{
		AnthropicAPIKey: "key",
		ActiveModel:     "claude-sonnet-4-20250514",
	}
	resolved := cfg.resolveActiveModel()
	if resolved != "claude-sonnet-4-20250514" {
		t.Errorf("resolveActiveModel = %q, want claude-sonnet-4-20250514", resolved)
	}
}

func TestResolveActiveModelMissingKeyFallback(t *testing.T) {
	// ActiveModel is an Anthropic model but no Anthropic key — should fall back
	cfg := Config{
		GrokAPIKey:  "key",
		ActiveModel: "claude-sonnet-4-20250514",
	}
	resolved := cfg.resolveActiveModel()
	// Should fall back to first available grok model
	if resolved == "claude-sonnet-4-20250514" {
		t.Error("should not resolve to a model whose provider has no key")
	}
	if resolved == "" {
		t.Error("should fall back to first available model")
	}
	m := findModelByID(resolved)
	if m == nil || m.Provider != ProviderGrok {
		t.Errorf("fallback should be a grok model, got %q", resolved)
	}
}

func TestResolveActiveModelEmptyConfig(t *testing.T) {
	cfg := Config{}
	resolved := cfg.resolveActiveModel()
	if resolved != "" {
		t.Errorf("resolveActiveModel with no keys = %q, want empty", resolved)
	}
}

func TestResolveActiveModelInvalidID(t *testing.T) {
	cfg := Config{
		OpenAIAPIKey: "key",
		ActiveModel:  "nonexistent-model",
	}
	resolved := cfg.resolveActiveModel()
	// Should fall back to first available OpenAI model
	if resolved == "nonexistent-model" {
		t.Error("should not resolve to invalid model ID")
	}
	m := findModelByID(resolved)
	if m == nil || m.Provider != ProviderOpenAI {
		t.Errorf("fallback should be an openai model, got %q", resolved)
	}
}
