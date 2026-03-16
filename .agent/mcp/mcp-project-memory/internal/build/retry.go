package build

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/llm"
)

// CallWithRetries retries an LLM call with backoff and optional fallback.
// Respects context cancellation for graceful shutdown (Ctrl+C).
func CallWithRetries(
	ctx context.Context,
	primary domain.LLMCaller,
	fn func(domain.LLMCaller) (string, error),
	fallback domain.LLMCaller,
) (string, error) {
	maxRetries := config.RetryMaxRetries()
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Check for cancellation before each attempt
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("cancelled: %w", err)
		}

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
			wait := rateLimitWait(err, attempt)
			fmt.Fprintf(os.Stderr, "  rate limited, waiting %v...\n", wait)
			if err := sleepCtx(ctx, wait); err != nil {
				return "", fmt.Errorf("cancelled during rate limit wait: %w", err)
			}
		case errorTransient:
			wait := time.Duration(config.RetryTransientBaseWait()+attempt) * time.Second
			fmt.Fprintf(os.Stderr, "  transient error, retrying in %v...\n", wait)
			if err := sleepCtx(ctx, wait); err != nil {
				return "", fmt.Errorf("cancelled during retry wait: %w", err)
			}
		}
	}

	return "", fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

// sleepCtx sleeps for the given duration, returning early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
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

// rateLimitWait returns the wait duration for a rate limit error.
// Uses the Retry-After header from the server when available,
// falls back to exponential backoff.
func rateLimitWait(err error, attempt int) time.Duration {
	var apiErr *llm.APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return time.Duration(config.RetryRateLimitBaseWait()*int(math.Pow(2, float64(attempt)))) * time.Second
}
