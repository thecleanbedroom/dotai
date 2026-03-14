package config_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/config"
)

// Tests for previously uncovered constant-returning functions.

func TestFilterIgnorePaths(t *testing.T) {
	cfg := config.Load()
	// Default should be a string (possibly empty)
	_ = cfg.FilterIgnorePaths()
}

func TestRetryBaseWaits(t *testing.T) {
	got := config.RetryRateLimitBaseWait()
	if got <= 0 {
		t.Errorf("RetryRateLimitBaseWait should be positive, got %d", got)
	}

	got = config.RetryTransientBaseWait()
	if got <= 0 {
		t.Errorf("RetryTransientBaseWait should be positive, got %d", got)
	}
}

func TestConfidenceSummaryThresholds(t *testing.T) {
	st := config.ConfidenceSummaryThresholds()
	if len(st) == 0 {
		t.Error("ConfidenceSummaryThresholds should not be empty")
	}
}

func TestConfidenceTagsThresholds(t *testing.T) {
	tt := config.ConfidenceTagsThresholds()
	if len(tt) == 0 {
		t.Error("ConfidenceTagsThresholds should not be empty")
	}
}

func TestExtractionMinOutputTokens(t *testing.T) {
	got := config.ExtractionMinOutputTokens()
	if got <= 0 {
		t.Errorf("ExtractionMinOutputTokens should be positive, got %d", got)
	}
}

func TestSynthesisContextFillRatio(t *testing.T) {
	got := config.SynthesisContextFillRatio()
	if got <= 0 || got > 1 {
		t.Errorf("SynthesisContextFillRatio should be 0<x<=1, got %f", got)
	}
}

func TestSynthesisThinkingRatio(t *testing.T) {
	got := config.SynthesisThinkingRatio()
	if got <= 0 || got > 1 {
		t.Errorf("SynthesisThinkingRatio should be 0<x<=1, got %f", got)
	}
}
