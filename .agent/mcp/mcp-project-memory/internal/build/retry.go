package build

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/llm"
)

// CallWithRetries retries an LLM call with backoff and optional fallback.
// Uses a single error classifier (DRY) for both extraction and synthesis.
func CallWithRetries(
	primary domain.LLMCaller,
	fn func(domain.LLMCaller) (string, error),
	fallback domain.LLMCaller,
) (string, error) {
	maxRetries := config.RetryMaxRetries()
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		caller := primary
		if attempt >= maxRetries/2 && fallback != nil {
			caller = fallback
		}

		result, err := fn(caller)
		if err == nil {
			return result, nil
		}
		lastErr = err

		cat := classifyError(err)
		switch cat {
		case errorFatal:
			return "", fmt.Errorf("fatal error (attempt %d/%d): %w", attempt+1, maxRetries, err)
		case errorRateLimit:
			wait := time.Duration(config.RetryRateLimitBaseWait()*int(math.Pow(2, float64(attempt)))) * time.Second
			fmt.Printf("  rate limited, waiting %v...\n", wait)
			time.Sleep(wait)
		case errorTransient:
			wait := time.Duration(config.RetryTransientBaseWait()+attempt) * time.Second
			fmt.Printf("  transient error, retrying in %v...\n", wait)
			time.Sleep(wait)
		}
	}

	return "", fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

// errorCategory classifies errors for retry logic (DRY — single definition).
type errorCategory int

const (
	errorFatal     errorCategory = iota
	errorRateLimit
	errorTransient
)

func classifyError(err error) errorCategory {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsRateLimit() {
			return errorRateLimit
		}
		if apiErr.IsTransient() {
			return errorTransient
		}
	}
	return errorFatal
}
