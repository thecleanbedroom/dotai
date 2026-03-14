package config

import "time"

// Config holds all gateway tuning parameters.
// Defaults mirror the Python gateway CONFIG dict.
type Config struct {
	// Models maps intent aliases to full Gemini model strings.
	// Use tiers: lite, quick, fast, think, deep.
	Models map[string]string

	// ModelBuckets groups models that can substitute for each other in batch dispatch.
	ModelBuckets [][]string

	// MaxConcurrent is the max simultaneous execution slots per model alias.
	MaxConcurrent map[string]int

	// MaxQueue is the max total pending jobs per model alias.
	MaxQueue map[string]int

	// QueuePollInterval is how often queued jobs check for an open slot.
	QueuePollInterval time.Duration

	// InitialGapMs is the starting gap between request launches per model.
	InitialGapMs map[string]int

	// FloorMs is the fastest allowed gap — adaptive algorithm won't go below this.
	FloorMs map[string]int

	// CeilingMs is the slowest gap after repeated rate-limits.
	CeilingMs int

	// JitterMs is the random jitter range added to each wait [min, max].
	JitterMs [2]int

	// SpeedupFactor multiplied on gap after each success (< 1.0 = faster).
	SpeedupFactor float64

	// SlowdownFactor multiplied on gap after each rate-limit (> 1.0 = slower).
	SlowdownFactor float64

	// BackoffInitialMs is the initial penalty added after a rate-limit.
	BackoffInitialMs int

	// BackoffMaxMs caps the maximum backoff penalty.
	BackoffMaxMs int

	// StreakThreshold is the number of consecutive successes before aggressive speedup.
	StreakThreshold int

	// StreakSpeedup is the stronger speedup factor during a success streak.
	StreakSpeedup float64

	// MaxRetries is the maximum retry attempts for rate-limited requests.
	MaxRetries int

	// TimeoutSeconds is the default subprocess timeout.
	TimeoutSeconds int

	// RateLimitSignals are strings in stdout/stderr that indicate a rate-limit.
	RateLimitSignals []string

	// RateLimitExitCode is the exit code Gemini CLI uses for rate-limit self-cancel.
	RateLimitExitCode int

	// CleanupDays auto-deletes completed/failed requests older than this.
	CleanupDays int

	// DBPath is the SQLite database path (relative to binary or absolute).
	DBPath string

	// SystemPrefix is prepended to every user prompt sent to Gemini CLI.
	SystemPrefix string
}

// Default returns a Config with all values matching the Python gateway defaults.
func Default() *Config {
	return &Config{
		Models: map[string]string{
			"lite":  "gemini-2.5-flash-lite",
			"quick": "gemini-2.5-flash",
			"fast":  "gemini-3-flash-preview",
			"think": "gemini-2.5-pro",
			"deep":  "gemini-3.1-pro-preview",
		},
		ModelBuckets: [][]string{
			{"lite", "quick", "fast"},
			{"think", "deep"},
		},
		MaxConcurrent: map[string]int{
			"lite": 1, "quick": 1, "fast": 1, "think": 1, "deep": 1,
		},
		MaxQueue: map[string]int{
			"lite": 50, "quick": 50, "fast": 50, "think": 50, "deep": 50,
		},
		QueuePollInterval: 3 * time.Second,
		InitialGapMs: map[string]int{
			"lite": 1500, "quick": 2000, "fast": 2000, "think": 3000, "deep": 3000,
		},
		FloorMs: map[string]int{
			"lite": 1000, "quick": 1500, "fast": 1500, "think": 2000, "deep": 2000,
		},
		CeilingMs:        10000,
		JitterMs:         [2]int{0, 250},
		SpeedupFactor:    0.90,
		SlowdownFactor:   1.3,
		BackoffInitialMs: 1500,
		BackoffMaxMs:     8000,
		StreakThreshold:   3,
		StreakSpeedup:     0.85,
		MaxRetries:       3,
		TimeoutSeconds:   420,
		RateLimitSignals: []string{
			"429",
			"RESOURCE_EXHAUSTED",
			"rate limit",
			"quota",
			"exhausted your capacity",
		},
		RateLimitExitCode: 130,
		CleanupDays:       7,
		DBPath:            "data/gateway.sqlite",
		SystemPrefix: "You are a code generation subagent dispatched by an orchestrating agent. " +
			"The orchestrator will review your work via `git diff` after you finish.\n\n" +
			"Tool usage:\n" +
			"- Read source code, existing tests, interfaces, and project conventions before writing.\n" +
			"- Write files directly using your file-writing tools. Create or modify files as needed.\n" +
			"- CRITICAL: Only write files within the current working directory. Do NOT write outside the codebase.\n\n" +
			"Output format:\n" +
			"- Write code directly to the target file paths.\n" +
			"- Include all necessary imports, namespace declarations, and use statements.\n" +
			"- Do NOT add explanations or commentary — just write the files.\n" +
			"---\n",
	}
}

// FloorForAlias returns the floor_ms for a model alias, defaulting to 800.
func (c *Config) FloorForAlias(alias string) int {
	if v, ok := c.FloorMs[alias]; ok {
		return v
	}
	return 800
}

// InitialGapForAlias returns the initial_gap_ms for a model alias, defaulting to 1200.
func (c *Config) InitialGapForAlias(alias string) int {
	if v, ok := c.InitialGapMs[alias]; ok {
		return v
	}
	return 1200
}
