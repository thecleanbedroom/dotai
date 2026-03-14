package config_test

import (
	"os"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/config"
)

func TestSettings_Defaults(t *testing.T) {
	cfg := config.Load()

	if got := cfg.APIURL(); got != "https://openrouter.ai/api/v1/chat/completions" {
		t.Errorf("APIURL: got %q", got)
	}
	if got := cfg.BatchingCommitLimit(); got != 0 {
		t.Errorf("BatchingCommitLimit: got %d", got)
	}
	if got := cfg.BatchingTokenBudget(); got != 100000 {
		t.Errorf("BatchingTokenBudget: got %d", got)
	}
	if got := cfg.BatchingMaxCommits(); got != 20 {
		t.Errorf("BatchingMaxCommits: got %d", got)
	}
	if got := cfg.ModelMinContextLength(); got != 32000 {
		t.Errorf("ModelMinContextLength: got %d", got)
	}
}

func TestSettings_EnvOverride(t *testing.T) {
	os.Setenv("MEMORY_BATCH_MAX_COMMITS", "50")
	defer os.Unsetenv("MEMORY_BATCH_MAX_COMMITS")

	cfg := config.Load()
	if got := cfg.BatchingMaxCommits(); got != 50 {
		t.Errorf("BatchingMaxCommits with env: got %d", got)
	}
}

func TestSettings_WithOverrides(t *testing.T) {
	cfg := config.Load()
	overridden := cfg.WithOverrides(map[string]string{
		"MEMORY_BATCH_TOKEN_BUDGET": "50000",
		"OPENROUTER_API_KEY":        "test-key",
	})

	if got := overridden.BatchingTokenBudget(); got != 50000 {
		t.Errorf("BatchingTokenBudget: got %d", got)
	}
	if got := overridden.APIKey(); got != "test-key" {
		t.Errorf("APIKey: got %q", got)
	}
	// Original should be unchanged
	if got := cfg.BatchingTokenBudget(); got != 100000 {
		t.Errorf("original BatchingTokenBudget: got %d", got)
	}
}

func TestSettings_InvalidInt(t *testing.T) {
	os.Setenv("MEMORY_BATCH_MAX_COMMITS", "not-a-number")
	defer os.Unsetenv("MEMORY_BATCH_MAX_COMMITS")

	cfg := config.Load()
	// Should fall back to default
	if got := cfg.BatchingMaxCommits(); got != 20 {
		t.Errorf("expected default 20 for invalid int, got %d", got)
	}
}

func TestSettings_Models(t *testing.T) {
	cfg := config.Load()
	if got := cfg.ExtractionModel(); got == "" {
		t.Error("ExtractionModel should have a default")
	}
	if got := cfg.ExtractionFallbackModel(); got == "" {
		t.Error("ExtractionFallbackModel should have a default")
	}
	if got := cfg.SynthesisModel(); got == "" {
		t.Error("SynthesisModel should have a default")
	}
}

func TestInternalSettings_EstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},         // minimum 1
		{"abcd", 1},     // 4 chars = 1 token
		{"12345678", 2}, // 8 chars = 2 tokens
		{string(make([]byte, 400)), 100},
	}

	for _, tt := range tests {
		got := config.EstimateTokens(tt.input)
		if got != tt.expected {
			t.Errorf("EstimateTokens(%d chars): got %d, want %d", len(tt.input), got, tt.expected)
		}
	}
}

func TestInternalSettings_Constants(t *testing.T) {
	if config.TokenCharsPerToken() != 4 {
		t.Errorf("TokenCharsPerToken: got %d", config.TokenCharsPerToken())
	}
	if config.RetryMaxRetries() != 4 {
		t.Errorf("RetryMaxRetries: got %d", config.RetryMaxRetries())
	}
	if config.ExtractionOverheadTokens() != 8000 {
		t.Errorf("ExtractionOverheadTokens: got %d", config.ExtractionOverheadTokens())
	}
	if config.ValidationMinCommitHashLength() != 4 {
		t.Errorf("ValidationMinCommitHashLength: got %d", config.ValidationMinCommitHashLength())
	}
	if config.ValidationMaxImportance() != 100 {
		t.Errorf("ValidationMaxImportance: got %d", config.ValidationMaxImportance())
	}
}

func TestInternalSettings_Thresholds(t *testing.T) {
	ct := config.ConfidenceCommitsThresholds()
	if len(ct) == 0 {
		t.Error("ConfidenceCommitsThresholds should not be empty")
	}
	ft := config.ConfidenceFilesThresholds()
	if len(ft) == 0 {
		t.Error("ConfidenceFilesThresholds should not be empty")
	}
}

func TestInternalSettings_ExtractionPrompt(t *testing.T) {
	prompt := config.ExtractionSystemPrompt()
	if prompt == "" {
		t.Error("ExtractionSystemPrompt should not be empty")
	}
}

func TestInternalSettings_ExtractionSchema(t *testing.T) {
	schema := config.ExtractionSchema()
	if len(schema) == 0 {
		t.Error("ExtractionSchema should not be empty")
	}
}

func TestInternalSettings_SynthesisPrompts(t *testing.T) {
	if config.SynthesisTriagePrompt() == "" {
		t.Error("SynthesisTriagePrompt should not be empty")
	}
	if config.SynthesisLinkingPrompt() == "" {
		t.Error("SynthesisLinkingPrompt should not be empty")
	}
}

func TestInternalSettings_SynthesisSchemas(t *testing.T) {
	if len(config.SynthesisTriageSchema()) == 0 {
		t.Error("SynthesisTriageSchema should not be empty")
	}
	if len(config.SynthesisLinkingSchema()) == 0 {
		t.Error("SynthesisLinkingSchema should not be empty")
	}
}
