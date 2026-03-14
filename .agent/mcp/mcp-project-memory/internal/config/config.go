// Package config loads user-configurable settings from .env files and
// environment variables.
package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Settings holds user-configurable values. Each method checks for an
// environment variable, then returns a default.
type Settings struct {
	overrides map[string]string
}

// Load creates a Settings instance. It loads .env from envPath (if provided)
// or tries the default .env location.
func Load(envPaths ...string) *Settings {
	for _, p := range envPaths {
		_ = godotenv.Load(p)
	}
	return &Settings{overrides: make(map[string]string)}
}

// WithOverrides returns a copy with additional overrides (for testing).
func (s *Settings) WithOverrides(overrides map[string]string) *Settings {
	merged := make(map[string]string, len(s.overrides)+len(overrides))
	for k, v := range s.overrides {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return &Settings{overrides: merged}
}

func (s *Settings) env(key, defaultVal string) string {
	if v, ok := s.overrides[key]; ok {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func (s *Settings) envInt(key string, defaultVal int) int {
	raw := s.env(key, "")
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// --- API ---

func (s *Settings) APIKey() string {
	return s.env("OPENROUTER_API_KEY", "")
}

func (s *Settings) APIURL() string {
	return s.env("MEMORY_BUILD_API_URL", "https://openrouter.ai/api/v1/chat/completions")
}

// --- Extraction (Pass 1) ---

func (s *Settings) ExtractionModel() string {
	return s.env("MEMORY_EXTRACT_MODEL", "nvidia/nemotron-3-super-120b-a12b:free")
}

func (s *Settings) ExtractionFallbackModel() string {
	return s.env("MEMORY_EXTRACT_FALLBACK_MODEL", "google/gemini-2.5-flash-lite")
}

// --- Synthesis (Pass 2) ---

func (s *Settings) SynthesisModel() string {
	return s.env("MEMORY_REASONING_MODEL", "google/gemini-3.1-pro-preview")
}

// --- Batching ---

func (s *Settings) BatchingCommitLimit() int {
	return s.envInt("MEMORY_COMMIT_LIMIT", 0)
}

func (s *Settings) BatchingTokenBudget() int {
	return s.envInt("MEMORY_BATCH_TOKEN_BUDGET", 100000)
}

func (s *Settings) BatchingMaxCommits() int {
	return s.envInt("MEMORY_BATCH_MAX_COMMITS", 20)
}

// --- Model constraints ---

func (s *Settings) ModelMinContextLength() int {
	return s.envInt("MIN_CONTEXT_LENGTH", 32000)
}

// --- Path filtering ---

func (s *Settings) FilterIgnorePaths() string {
	return s.env("MEMORY_IGNORE_PATHS", ".agent/memory/data/*")
}
