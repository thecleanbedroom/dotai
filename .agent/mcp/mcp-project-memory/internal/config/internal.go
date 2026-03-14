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

func TokenCharsPerToken() int { return 4 }

// --- Extraction (Pass 1) ---

func ExtractionOverheadTokens() int    { return 8_000 }
func ExtractionMinOutputTokens() int   { return 4_000 }

func ExtractionSystemPrompt() string {
	return loadText("extraction_prompt", "prompts/prompt_extract_system.md", promptsFS) + "\n\n" + jsonRules()
}

func ExtractionSchema() json.RawMessage {
	return loadJSON("extraction_schema", "schemas/schema_extract.json", schemasFS)
}

// --- Synthesis (Pass 2) ---

func SynthesisContextFillRatio() float64 { return 0.5 }
func SynthesisThinkingRatio() float64    { return 0.15 }

func SynthesisTriagePrompt() string {
	return loadText("triage_prompt", "prompts/prompt_synthesis_triage.md", promptsFS) + "\n\n" + jsonRules()
}

func SynthesisTriageSchema() json.RawMessage {
	return loadJSON("triage_schema", "schemas/schema_synthesis_triage.json", schemasFS)
}

func SynthesisLinkingPrompt() string {
	return loadText("linking_prompt", "prompts/prompt_synthesis_linking.md", promptsFS) + "\n\n" + jsonRules()
}

func SynthesisLinkingSchema() json.RawMessage {
	return loadJSON("linking_schema", "schemas/schema_synthesis_linking.json", schemasFS)
}

// --- Retry / resilience ---

func RetryMaxRetries() int         { return 4 }
func RetryRateLimitBaseWait() int  { return 15 }
func RetryTransientBaseWait() int  { return 1 }

// --- Confidence scoring thresholds ---

func ConfidenceCommitsThresholds() map[int]int {
	return map[int]int{1: 8, 2: 20, 3: 30}
}

func ConfidenceFilesThresholds() map[int]int {
	return map[int]int{1: 5, 2: 15, 4: 25, 7: 30}
}

func ConfidenceSummaryThresholds() map[int]int {
	return map[int]int{100: 5, 200: 12, 300: 20}
}

func ConfidenceTagsThresholds() map[int]int {
	return map[int]int{3: 5, 5: 12, 7: 20}
}

// --- Validation ---

func ValidationMinCommitHashLength() int { return 4 }
func ValidationMaxImportance() int       { return 100 }

// EstimateTokens returns a rough token count from character length.
// Defined here (DRY) — used by batching, synthesis, and anywhere else.
func EstimateTokens(text string) int {
	n := len(text) / TokenCharsPerToken()
	if n < 1 {
		return 1
	}
	return n
}
