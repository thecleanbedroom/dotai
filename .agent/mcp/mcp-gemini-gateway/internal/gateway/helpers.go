package gateway

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
)

// PromptHash returns the first 12 hex chars of the SHA256 of a prompt.
func PromptHash(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", h)[:12]
}

// DetectRateLimit checks if a result indicates a rate-limit.
func DetectRateLimit(cfg *config.Config, exitCode int, stdout, stderr string) bool {
	if exitCode == cfg.RateLimitExitCode {
		return true
	}
	combined := strings.ToLower(stdout + stderr)
	for _, sig := range cfg.RateLimitSignals {
		if strings.Contains(combined, strings.ToLower(sig)) {
			return true
		}
	}
	return false
}

// ParseDuration parses a --last style duration string (e.g., "1h", "2d", "30m").
// Returns 0 for empty input (meaning "lifetime").
func ParseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(strings.ToLower(s))

	if len(s) < 2 {
		// Try parsing as hours
		if v, err := parseFloat(s); err == nil {
			return time.Duration(v * float64(time.Hour))
		}
		return 0
	}

	suffix := s[len(s)-1]
	numStr := s[:len(s)-1]
	v, err := parseFloat(numStr)
	if err != nil {
		return 0
	}

	switch suffix {
	case 'h':
		return time.Duration(v * float64(time.Hour))
	case 'd':
		return time.Duration(v * 24 * float64(time.Hour))
	case 'm':
		return time.Duration(v * float64(time.Minute))
	default:
		// Default to hours
		if vFull, err := parseFloat(s); err == nil {
			return time.Duration(vFull * float64(time.Hour))
		}
		return 0
	}
}

func parseFloat(s string) (float64, error) {
	var v float64
	_, err := fmt.Sscanf(s, "%f", &v)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("invalid number: %s", s)
	}
	return v, nil
}
