package llm_test

import (
	"testing"
	"time"

	"github.com/dotai/mcp-project-memory/internal/llm"
)

func TestRateLimiter_Acquire(t *testing.T) {
	rl := llm.NewRateLimiter(60) // 60 RPM

	// Should not block on first call
	start := time.Now()
	rl.Acquire()
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("first acquire should be instant, took %v", elapsed)
	}
}

func TestRateLimiter_OnSuccess(t *testing.T) {
	rl := llm.NewRateLimiter(60)
	rl.Acquire()
	rl.OnSuccess() // should not panic
}

func TestRateLimiter_OnRateLimit(t *testing.T) {
	rl := llm.NewRateLimiter(60)
	rl.OnRateLimit(1) // 1 second backoff

	start := time.Now()
	rl.Acquire()
	elapsed := time.Since(start)
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected ~1s wait after rate limit, got %v", elapsed)
	}
}

func TestRateLimiter_OnRateLimit_DefaultWait(t *testing.T) {
	rl := llm.NewRateLimiter(60)
	rl.OnRateLimit(0) // should default to 15s, but we won't wait that long
	// Just verify it doesn't panic
}
