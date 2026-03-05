package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Provider constants for supported AI providers.
const (
	ProviderAnthropic = "anthropic"
	ProviderGrok      = "grok"
	ProviderOpenAI    = "openai"
	ProviderGemini    = "gemini"
)

// ModelDef describes a model available for selection.
// IDs are native API model identifiers (not OpenRouter format).
type ModelDef struct {
	Provider        string
	ID              string
	DisplayName     string
	PromptPrice     float64 // USD per million input tokens
	CompletionPrice float64 // USD per million output tokens
	SWEScore        float64 // SWE-bench Verified score (0 = no data)
}

// builtinModels returns the hardcoded list of supported models with native API IDs and prices.
func builtinModels() []ModelDef {
	return []ModelDef{
		// Anthropic
		{ProviderAnthropic, "claude-opus-4-1-20250620", "Claude Opus 4.1", 15.0, 75.0, 0},
		{ProviderAnthropic, "claude-opus-4-0-20250514", "Claude Opus 4", 15.0, 75.0, 0},
		{ProviderAnthropic, "claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet", 6.0, 30.0, 0},
		{ProviderAnthropic, "claude-opus-4-6-20250801", "Claude Opus 4.6", 5.0, 25.0, 0},
		{ProviderAnthropic, "claude-opus-4-5-20250620", "Claude Opus 4.5", 5.0, 25.0, 0},
		{ProviderAnthropic, "claude-sonnet-4-6-20250801", "Claude Sonnet 4.6", 3.0, 15.0, 0},
		{ProviderAnthropic, "claude-sonnet-4-5-20250514", "Claude Sonnet 4.5", 3.0, 15.0, 0},
		{ProviderAnthropic, "claude-sonnet-4-0-20250514", "Claude Sonnet 4", 3.0, 15.0, 0},
		{ProviderAnthropic, "claude-3-7-sonnet-20250219", "Claude 3.7 Sonnet", 3.0, 15.0, 0},
		{ProviderAnthropic, "claude-haiku-4-5-20250414", "Claude Haiku 4.5", 1.0, 5.0, 0},
		{ProviderAnthropic, "claude-3-5-haiku-20241022", "Claude 3.5 Haiku", 0.80, 4.0, 0},
		{ProviderAnthropic, "claude-3-haiku-20240307", "Claude 3 Haiku", 0.25, 1.25, 0},

		// Grok (x.ai)
		{ProviderGrok, "grok-4", "Grok 4", 3.0, 15.0, 0},
		{ProviderGrok, "grok-3", "Grok 3", 3.0, 15.0, 0},
		{ProviderGrok, "grok-3-beta", "Grok 3 Beta", 3.0, 15.0, 0},
		{ProviderGrok, "grok-3-mini", "Grok 3 Mini", 0.30, 0.50, 0},
		{ProviderGrok, "grok-3-mini-beta", "Grok 3 Mini Beta", 0.30, 0.50, 0},
		{ProviderGrok, "grok-4-1-fast", "Grok 4.1 Fast", 0.20, 0.50, 0},
		{ProviderGrok, "grok-4-fast", "Grok 4 Fast", 0.20, 0.50, 0},
		{ProviderGrok, "grok-code-fast-1", "Grok Code Fast 1", 0.20, 1.50, 0},

		// OpenAI
		{ProviderOpenAI, "gpt-4o", "GPT-4o", 2.50, 10.0, 0},
		{ProviderOpenAI, "gpt-4o-mini", "GPT-4o Mini", 0.15, 0.60, 0},
		{ProviderOpenAI, "o3-mini", "o3-mini", 1.10, 4.40, 0},

		// Gemini
		{ProviderGemini, "gemini-2.5-pro", "Gemini 2.5 Pro", 1.25, 10.0, 0},
		{ProviderGemini, "gemini-2.5-flash", "Gemini 2.5 Flash", 0.15, 0.60, 0},
	}
}

// filterModelsByProviders returns models whose provider is in the given set.
func filterModelsByProviders(models []ModelDef, providers map[string]bool) []ModelDef {
	var result []ModelDef
	for _, m := range models {
		if providers[m.Provider] {
			result = append(result, m)
		}
	}
	return result
}

// findModelByID returns the model with the given ID, or nil if not found.
func findModelByID(models []ModelDef, id string) *ModelDef {
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}

// formatPrice formats a per-million-token price as "$X.XX".
func formatPrice(price float64) string {
	return fmt.Sprintf("$%.2f", price)
}

// SWE-bench leaderboard types

const sweBenchURL = "https://raw.githubusercontent.com/SWE-bench/swe-bench.github.io/master/data/leaderboards.json"

type sweBenchResponse struct {
	Leaderboards []sweBenchLeaderboard `json:"leaderboards"`
}

type sweBenchLeaderboard struct {
	Name    string           `json:"name"`
	Results []sweBenchResult `json:"results"`
}

type sweBenchResult struct {
	Name     string   `json:"name"`
	Resolved float64  `json:"resolved"`
	Tags     []string `json:"tags"`
}

// parseSWEScores extracts the highest SWE-bench Verified score per model tag
// from leaderboard results. Returns a map from model tag identifier (e.g.
// "claude-opus-4-5-20251101") to the best resolved score.
func parseSWEScores(resp sweBenchResponse) map[string]float64 {
	scores := make(map[string]float64)
	for _, lb := range resp.Leaderboards {
		if lb.Name != "Verified" {
			continue
		}
		for _, r := range lb.Results {
			var modelTags []string
			for _, tag := range r.Tags {
				if strings.HasPrefix(tag, "Model: ") {
					modelTags = append(modelTags, strings.TrimPrefix(tag, "Model: "))
				}
			}
			// Skip entries with multiple model tags (multi-model systems)
			if len(modelTags) != 1 {
				continue
			}
			tag := modelTags[0]
			if r.Resolved > scores[tag] {
				scores[tag] = r.Resolved
			}
		}
		break // only process "Verified"
	}
	return scores
}

// matchSWEScores enriches models with SWE-bench scores by fuzzy-matching
// model IDs against SWE-bench model tags.
func matchSWEScores(models []ModelDef, scores map[string]float64) {
	for i := range models {
		id := models[i].ID
		// Try exact match first, then check if either contains the other
		for tag, score := range scores {
			if tag == id || strings.Contains(tag, id) || strings.Contains(id, tag) {
				if score > models[i].SWEScore {
					models[i].SWEScore = score
				}
			}
		}
	}
}

// fetchSWEScores fetches the SWE-bench Verified leaderboard and returns
// a map of model tag identifiers to their best scores.
func fetchSWEScores() (map[string]float64, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(sweBenchURL)
	if err != nil {
		return nil, fmt.Errorf("fetching SWE-bench scores: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SWE-bench API returned status %d", resp.StatusCode)
	}

	var body sweBenchResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding SWE-bench response: %w", err)
	}

	return parseSWEScores(body), nil
}
