package llm

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// OpenRouter provides model info, pricing, rate limits, and cost estimation.
type OpenRouter struct {
	apiURL string
	apiKey string
	mu     sync.Mutex
	cache  map[string]domain.ModelInfo
}

// NewOpenRouter creates an OpenRouter client.
func NewOpenRouter(apiURL, apiKey string) *OpenRouter {
	return &OpenRouter{
		apiURL: apiURL,
		apiKey: apiKey,
		cache:  make(map[string]domain.ModelInfo),
	}
}

// GetModelInfo fetches and caches model metadata from OpenRouter.
func (o *OpenRouter) GetModelInfo(model string) (domain.ModelInfo, error) {
	o.mu.Lock()
	if info, ok := o.cache[model]; ok {
		o.mu.Unlock()
		return info, nil
	}
	o.mu.Unlock()

	// Fetch from OpenRouter API
	baseURL := strings.TrimSuffix(o.apiURL, "/chat/completions")
	url := fmt.Sprintf("%s/models/%s", baseURL, model)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return domain.ModelInfo{}, fmt.Errorf("create request: %w", err)
	}
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return domain.ModelInfo{}, fmt.Errorf("fetch model info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.ModelInfo{}, fmt.Errorf("read model info body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return domain.ModelInfo{}, fmt.Errorf("model info %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			ContextLength       int      `json:"context_length"`
			MaxCompletionTokens int      `json:"top_provider_max_completion_tokens"`
			Pricing             struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			SupportedParameters []string `json:"supported_parameters"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		// Try alternate format
		var alt struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			ContextLength       int      `json:"context_length"`
			MaxCompletionTokens int      `json:"top_provider_max_completion_tokens"`
			Pricing             struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			SupportedParameters []string `json:"supported_parameters"`
		}
		if err := json.Unmarshal(body, &alt); err != nil {
			return domain.ModelInfo{}, fmt.Errorf("parse model info: %w", err)
		}
		result.Data = alt
	}

	d := result.Data
	info := domain.ModelInfo{
		Name:                d.Name,
		ContextLength:       d.ContextLength,
		MaxCompletionTokens: d.MaxCompletionTokens,
		SupportedParams:     d.SupportedParameters,
		Pricing:             map[string]float64{},
		IsFree:              d.Pricing.Prompt == "0" && d.Pricing.Completion == "0",
	}

	o.mu.Lock()
	o.cache[model] = info
	o.mu.Unlock()

	return info, nil
}

// EstimateCost estimates the cost for processing given token counts.
func (o *OpenRouter) EstimateCost(model string, inputTokens, outputTokens int) (float64, error) {
	info, err := o.GetModelInfo(model)
	if err != nil {
		return 0, err
	}

	promptPrice := info.Pricing["prompt"]
	completionPrice := info.Pricing["completion"]

	return (float64(inputTokens)*promptPrice + float64(outputTokens)*completionPrice) / 1_000_000, nil
}

// ListSuitableModels queries OpenRouter for all models (free and paid)
// meeting minContext and supporting JSON structured output (response_format).
func (o *OpenRouter) ListSuitableModels(minContext int) ([]domain.ModelInfo, error) {
	baseURL := strings.TrimSuffix(o.apiURL, "/chat/completions")
	url := baseURL + "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read models body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models API %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID                  string   `json:"id"`
			Name                string   `json:"name"`
			Description         string   `json:"description"`
			ContextLength       int      `json:"context_length"`
			SupportedParameters []string `json:"supported_parameters"`
			TopProvider         struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	var models []domain.ModelInfo
	for _, m := range result.Data {
		if m.ContextLength < minContext {
			continue
		}
		// Check for structured output support
		hasJSON := false
		for _, p := range m.SupportedParameters {
			if p == "response_format" || p == "json_mode" || p == "structured_output" {
				hasJSON = true
				break
			}
		}
		if !hasJSON {
			continue
		}

		// Skip models with no reported max_completion_tokens (null in API)
		if m.TopProvider.MaxCompletionTokens == 0 {
			continue
		}
		// Skip models with < 32K max output — extraction needs ~16K for
		// JSON memory arrays, and some models (deepseek-r1, o3) always
		// reason internally, eating into the completion budget. 32K gives
		// 2x margin for the observed ~16K extraction output.
		if m.TopProvider.MaxCompletionTokens < 32000 {
			continue
		}

		isFree := m.Pricing.Prompt == "0" && m.Pricing.Completion == "0"

		// Parse pricing as floats for cost comparison
		pricing := map[string]float64{}
		if pf, err := parseFloat(m.Pricing.Prompt); err == nil {
			pricing["prompt"] = pf
		}
		if cf, err := parseFloat(m.Pricing.Completion); err == nil {
			pricing["completion"] = cf
		}

		// Skip models with negative pricing (e.g. routing proxies)
		if pricing["prompt"] < 0 || pricing["completion"] < 0 {
			continue
		}

		info := domain.ModelInfo{
			Name:                m.Name,
			ID:                  m.ID,
			ContextLength:       m.ContextLength,
			MaxCompletionTokens: m.TopProvider.MaxCompletionTokens,
			SupportedParams:     m.SupportedParameters,
			Pricing:             pricing,
			IsFree:              isFree,
		}
		models = append(models, info)
	}

	// Sort by estimated build cost ascending (free = $0 naturally first).
	// Estimate: 100K input + 16K output per extraction batch.
	estimateCost := func(m domain.ModelInfo) float64 {
		return m.Pricing["prompt"]*100_000 + m.Pricing["completion"]*16_000
	}
	slices.SortFunc(models, func(a, b domain.ModelInfo) int {
		return cmp.Compare(estimateCost(a), estimateCost(b))
	})

	return models, nil
}

// AutoSelectExtractionModel picks the best model for extraction (Pass 1).
//
// Context window budget (extraction):
//
//	  model_context >= batch_token_budget + overhead + output
//	                   100K (default)    + 8K       + 16K     = ~124K
//
//	  ExtractionOverheadTokens()  = 8,000  (system prompt + schema)
//	  ExtractionMinOutputTokens() = 4,000  (minimum, but actual ~16K)
//	  BatchingTokenBudget()       = 100,000 (max input per batch)
//
//	  max_completion_tokens >= 16K (extraction output is structured JSON,
//	  no reasoning tokens — extraction doesn't use thinking)
//
// Priority: free > cheapest paid.
func (o *OpenRouter) AutoSelectExtractionModel(extractionMinCtx int) (domain.ModelInfo, error) {
	models, err := o.ListSuitableModels(extractionMinCtx)
	if err != nil {
		return domain.ModelInfo{}, err
	}

	// Free models first (already sorted by cost, free = $0 first)
	for _, m := range models {
		if m.IsFree {
			return m, nil
		}
	}
	// Cheapest paid model
	for _, m := range models {
		if !m.IsFree {
			return m, nil
		}
	}
	return domain.ModelInfo{}, fmt.Errorf("no suitable extraction model found (min context: %d)", extractionMinCtx)
}

// AutoSelectReasoningModel picks the best model for synthesis (Pass 2).
//
// Context window budget (synthesis):
//
//	  SynthesisContextFillRatio()  = 0.50 → 50% of context for input corpus
//	  SynthesisThinkingRatio()     = 0.15 → 15% reserved for reasoning tokens
//	  Remaining                    = 0.35 → 35% for output (triage/linking JSON)
//
//	  max_completion_tokens must cover BOTH reasoning + output:
//	    Observed: ~20K thinking + ~29K output = ~49K total
//	    With growth margin: 65K min completion tokens
//
//	  The ratios describe how we PLAN to fill context, not hard limits.
//	  A 65K completion cap handles current workloads with margin.
//
// Requires context >= synthesisMinCtx AND max_completion_tokens >= minOutput.
// Priority: free > cheapest paid.
func (o *OpenRouter) AutoSelectReasoningModel(synthesisMinCtx, minOutput int) (domain.ModelInfo, error) {
	models, err := o.ListSuitableModels(synthesisMinCtx)
	if err != nil {
		return domain.ModelInfo{}, err
	}

	// Filter to only models with explicitly reported sufficient output capacity.
	// MaxCompletionTokens == 0 means unknown (API returned null) — exclude.
	// Completion budget must cover reasoning tokens + JSON output.
	var candidates []domain.ModelInfo
	for _, m := range models {
		if m.MaxCompletionTokens >= minOutput {
			candidates = append(candidates, m)
		}
	}

	// Free models first, then cheapest paid
	for _, m := range candidates {
		if m.IsFree {
			return m, nil
		}
	}
	for _, m := range candidates {
		if !m.IsFree {
			return m, nil
		}
	}
	return domain.ModelInfo{}, fmt.Errorf("no suitable reasoning model found (min context: %d, min output: %d)", synthesisMinCtx, minOutput)
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
