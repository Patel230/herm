package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

//go:embed models.json
var modelsJSON []byte

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
	Provider        string  `json:"provider"`
	ID              string  `json:"id"`
	DisplayName     string  `json:"display_name"`
	PromptPrice     float64 `json:"prompt_price"`      // USD per million input tokens
	CompletionPrice float64 `json:"completion_price"`   // USD per million output tokens
	ContextWindow   int     `json:"context_window"`     // tokens
	SWEScore        float64 `json:"-"`                  // SWE-bench Verified score (0 = no data), populated at runtime
}

// builtinModels returns the list of supported models loaded from the embedded models.json.
func builtinModels() []ModelDef {
	var models []ModelDef
	if err := json.Unmarshal(modelsJSON, &models); err != nil {
		panic(fmt.Sprintf("failed to parse embedded models.json: %v", err))
	}
	return models
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
