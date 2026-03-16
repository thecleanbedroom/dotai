// Package config — internal non-configurable constants.
// Prompts and schemas are embedded at compile time via go:embed.
package config

import (
	"embed"
	"encoding/json"
	"sync"
)

//go:embed all:prompts
var promptsFS embed.FS

//go:embed all:schemas
var schemasFS embed.FS

// textCache and jsonCache prevent repeated file reads.
var (
	textCache   = map[string]string{}
	jsonCache   = map[string]json.RawMessage{}
	cacheMu     sync.Mutex
)

func loadText(key, path string, fs embed.FS) string {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if v, ok := textCache[key]; ok {
		return v
	}
	data, err := fs.ReadFile(path)
	if err != nil {
		return ""
	}
	textCache[key] = string(data)
	return textCache[key]
}

func loadJSON(key, path string, fs embed.FS) json.RawMessage {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if v, ok := jsonCache[key]; ok {
		return v
	}
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil
	}
	jsonCache[key] = json.RawMessage(data)
	return jsonCache[key]
}

func jsonRules() string {
	return loadText("json_rules", "prompts/prompt_json_rules.md", promptsFS)
}

// --- Token estimation ---

// TokenCharsPerToken returns the assumed characters-per-token ratio for estimation.
func TokenCharsPerToken() int { return 4 }

// --- Extraction (Pass 1) ---

// ExtractionOverheadTokens returns the token budget reserved for system prompt overhead.
func ExtractionOverheadTokens() int { return 8_000 }

// ExtractionMinOutputTokens returns the minimum output token budget for extraction.
func ExtractionMinOutputTokens() int { return 4_000 }

// ExtractionSystemPrompt returns the system prompt for memory extraction, with JSON rules appended.
func ExtractionSystemPrompt() string {
	return loadText("extraction_prompt", "prompts/prompt_extract_system.md", promptsFS) + "\n\n" + jsonRules()
}

// ExtractionSchema returns the JSON schema for extraction structured output.
func ExtractionSchema() json.RawMessage {
	return loadJSON("extraction_schema", "schemas/schema_extract.json", schemasFS)
}

// --- Synthesis (Pass 2) ---

// SynthesisContextFillRatio returns the fraction of context window to fill with corpus data.
func SynthesisContextFillRatio() float64 { return 0.5 }

// SynthesisThinkingRatio returns the fraction of output budget reserved for model thinking.
func SynthesisThinkingRatio() float64 { return 0.15 }

// SynthesisTriagePrompt returns the system prompt for memory triage during synthesis.
func SynthesisTriagePrompt() string {
	return loadText("triage_prompt", "prompts/prompt_synthesis_triage.md", promptsFS) + "\n\n" + jsonRules()
}

// SynthesisTriageSchema returns the JSON schema for triage structured output.
func SynthesisTriageSchema() json.RawMessage {
	return loadJSON("triage_schema", "schemas/schema_synthesis_triage.json", schemasFS)
}

// SynthesisLinkingPrompt returns the system prompt for memory linking during synthesis.
func SynthesisLinkingPrompt() string {
	return loadText("linking_prompt", "prompts/prompt_synthesis_linking.md", promptsFS) + "\n\n" + jsonRules()
}

// SynthesisLinkingSchema returns the JSON schema for linking structured output.
func SynthesisLinkingSchema() json.RawMessage {
	return loadJSON("linking_schema", "schemas/schema_synthesis_linking.json", schemasFS)
}

// --- Retry / resilience ---

// RetryMaxRetries returns the maximum number of LLM call retry attempts.
func RetryMaxRetries() int { return 4 }

// RetryRateLimitBaseWait returns the base wait (seconds) for rate-limit backoff.
func RetryRateLimitBaseWait() int { return 15 }

// RetryTransientBaseWait returns the base wait (seconds) for transient-error retries.
func RetryTransientBaseWait() int { return 1 }

// --- Confidence scoring thresholds ---

// ConfidenceCommitsThresholds returns source-commit count → confidence bonus mappings.
func ConfidenceCommitsThresholds() map[int]int {
	return map[int]int{1: 8, 2: 20, 3: 30}
}

// ConfidenceFilesThresholds returns file-count → confidence bonus mappings.
func ConfidenceFilesThresholds() map[int]int {
	return map[int]int{1: 5, 2: 15, 4: 25, 7: 30}
}

// ConfidenceSummaryThresholds returns summary-length (chars) → confidence bonus mappings.
func ConfidenceSummaryThresholds() map[int]int {
	return map[int]int{100: 5, 200: 12, 300: 20}
}

// ConfidenceTagsThresholds returns tag-count → confidence bonus mappings.
func ConfidenceTagsThresholds() map[int]int {
	return map[int]int{3: 5, 5: 12, 7: 20}
}

// --- Validation ---

// ValidationMinCommitHashLength returns the minimum accepted commit hash length.
func ValidationMinCommitHashLength() int { return 4 }

// ValidationMaxImportance returns the maximum valid importance score (0–100 range).
func ValidationMaxImportance() int { return 100 }

// EstimateTokens returns a rough token count from character length.
// Defined here (DRY) — used by batching, synthesis, and anywhere else.
func EstimateTokens(text string) int {
	n := len(text) / TokenCharsPerToken()
	if n < 1 {
		return 1
	}
	return n
}
