package llm

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token-bucket rate limiter for LLM API calls.
type RateLimiter struct {
	mu        sync.Mutex
	rpm       int
	available int
	last      time.Time
	waitUntil time.Time
}

// NewRateLimiter creates a rate limiter with the given requests-per-minute.
func NewRateLimiter(rpm int) *RateLimiter {
	return &RateLimiter{
		rpm:       rpm,
		available: rpm,
		last:      time.Now(),
	}
}

// Acquire blocks until a request slot is available.
func (rl *RateLimiter) Acquire() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Wait if we're in a rate-limit cooldown
	if wait := time.Until(rl.waitUntil); wait > 0 {
		time.Sleep(wait)
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(rl.last).Seconds()
	refill := int(elapsed * float64(rl.rpm) / 60.0)
	if refill > 0 {
		rl.available += refill
		if rl.available > rl.rpm {
			rl.available = rl.rpm
		}
		rl.last = now
	}

	// Wait if no tokens available
	if rl.available <= 0 {
		wait := time.Duration(60.0/float64(rl.rpm)*1000) * time.Millisecond
		time.Sleep(wait)
		rl.available = 1
	}

	rl.available--
}

// OnSuccess records a successful call (no-op for now).
func (rl *RateLimiter) OnSuccess() {}

// OnRateLimit handles a 429 response by backing off.
func (rl *RateLimiter) OnRateLimit(retryAfterSecs int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if retryAfterSecs <= 0 {
		retryAfterSecs = 15
	}
	rl.waitUntil = time.Now().Add(time.Duration(retryAfterSecs) * time.Second)
	rl.available = 0
}
