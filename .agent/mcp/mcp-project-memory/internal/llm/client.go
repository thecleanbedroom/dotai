// Package llm implements domain.LLMCaller using the OpenRouter HTTP API.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// Client implements domain.LLMCaller via OpenRouter chat completions.
type Client struct {
	apiURL  string
	apiKey  string
	model   string
	router  *OpenRouter
	logDir  string
}

// llmHTTPClient is the shared HTTP client for LLM API calls.
// Package-level to enable connection reuse; 120s timeout prevents indefinite hangs.
var llmHTTPClient = &http.Client{Timeout: 120 * time.Second}

// NewClient creates an LLM client for the given model.
func NewClient(apiURL, apiKey, model, logDir string) *Client {
	return &Client{
		apiURL: apiURL,
		apiKey: apiKey,
		model:  model,
		router: NewOpenRouter(apiURL, apiKey),
		logDir: logDir,
	}
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat    `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat sends messages to the LLM and returns the response content.
func (c *Client) Chat(messages []domain.Message, opts domain.ChatOpts) (string, error) {
	reqMessages := make([]chatMessage, len(messages))
	for i, m := range messages {
		reqMessages[i] = chatMessage{Role: m.Role, Content: m.Content}
	}

	req := chatRequest{
		Model:    c.model,
		Messages: reqMessages,
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	if opts.ResponseSchema != nil {
		req.ResponseFormat = &responseFormat{
			Type:       "json_schema",
			JSONSchema: opts.ResponseSchema,
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	start := time.Now()
	resp, err := llmHTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// Log exchange
	c.logExchange(opts.Label, body, respBody, time.Since(start), resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
		// Capture Retry-After header for rate limit responses
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				apiErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return "", apiErr
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return "", &APIError{
			StatusCode: chatResp.Error.Code,
			Body:       chatResp.Error.Message,
		}
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// GetModelInfo returns info about the configured model.
func (c *Client) GetModelInfo() (domain.ModelInfo, error) {
	return c.router.GetModelInfo(c.model)
}

func (c *Client) logExchange(label string, reqBody, respBody []byte, duration time.Duration, statusCode int) {
	if c.logDir == "" {
		return
	}
	if err := os.MkdirAll(c.logDir, 0o755); err != nil {
		slog.Warn("llm log: mkdir", "err", err)
		return
	}

	ts := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s/%s_%s.json", c.logDir, ts, label)

	entry := map[string]any{
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"model":       c.model,
		"label":       label,
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}

	var reqMap, respMap map[string]any
	if err := json.Unmarshal(reqBody, &reqMap); err != nil {
		slog.Warn("llm log: unmarshal request", "err", err)
	}
	if err := json.Unmarshal(respBody, &respMap); err != nil {
		slog.Warn("llm log: unmarshal response", "err", err)
	}
	entry["request"] = reqMap
	entry["response"] = respMap

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		slog.Warn("llm log: marshal", "err", err)
		return
	}
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		slog.Warn("llm log: write", "err", err)
	}
}

// APIError represents an HTTP API error from OpenRouter.
type APIError struct {
	StatusCode int
	Body       string
	RetryAfter time.Duration // from Retry-After header, 0 if not present
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// IsRateLimit returns true if this is a 429 rate limit error.
func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == 429
}

// IsTransient returns true if this is a retryable server error.
func (e *APIError) IsTransient() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}
