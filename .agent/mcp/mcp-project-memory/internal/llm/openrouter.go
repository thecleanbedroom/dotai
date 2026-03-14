package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	body, _ := io.ReadAll(resp.Body)
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
