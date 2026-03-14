package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "AllModelsPopulated",
			check: func(t *testing.T, cfg *Config) {
				expected := []string{"lite", "quick", "fast", "think", "deep"}
				for _, alias := range expected {
					if model, ok := cfg.Models[alias]; !ok || model == "" {
						t.Errorf("model alias %q is missing or empty", alias)
					}
				}
			},
		},
		{
			name: "PacingParamsNonZero",
			check: func(t *testing.T, cfg *Config) {
				if cfg.SpeedupFactor <= 0 || cfg.SpeedupFactor >= 1 {
					t.Errorf("SpeedupFactor=%f, want (0, 1)", cfg.SpeedupFactor)
				}
				if cfg.SlowdownFactor <= 1 {
					t.Errorf("SlowdownFactor=%f, want > 1", cfg.SlowdownFactor)
				}
				if cfg.CeilingMs <= 0 {
					t.Errorf("CeilingMs=%d, want > 0", cfg.CeilingMs)
				}
				for alias, floor := range cfg.FloorMs {
					if floor <= 0 {
						t.Errorf("FloorMs[%s]=%d, want > 0", alias, floor)
					}
				}
			},
		},
		{
			name: "MaxConcurrentAndQueue",
			check: func(t *testing.T, cfg *Config) {
				for alias, v := range cfg.MaxConcurrent {
					if v <= 0 {
						t.Errorf("MaxConcurrent[%s]=%d, want > 0", alias, v)
					}
				}
				for alias, v := range cfg.MaxQueue {
					if v <= 0 {
						t.Errorf("MaxQueue[%s]=%d, want > 0", alias, v)
					}
				}
			},
		},
		{
			name: "RateLimitSignals",
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.RateLimitSignals) == 0 {
					t.Error("RateLimitSignals should not be empty")
				}
			},
		},
		{
			name: "ModelBuckets",
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.ModelBuckets) != 2 {
					t.Errorf("len(ModelBuckets)=%d, want 2", len(cfg.ModelBuckets))
				}
			},
		},
		{
			name: "SystemPrefix",
			check: func(t *testing.T, cfg *Config) {
				if cfg.SystemPrefix == "" {
					t.Error("SystemPrefix should not be empty")
				}
			},
		},
	}

	cfg := Default()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check(t, cfg)
		})
	}
}

func TestFloorForAlias(t *testing.T) {
	t.Parallel()
	cfg := Default()

	tests := []struct {
		alias string
		want  int
	}{
		{"fast", cfg.FloorMs["fast"]},
		{"think", cfg.FloorMs["think"]},
		{"nonexistent", 800}, // default
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			t.Parallel()
			got := cfg.FloorForAlias(tt.alias)
			if got != tt.want {
				t.Errorf("FloorForAlias(%q)=%d, want %d", tt.alias, got, tt.want)
			}
		})
	}
}

func TestInitialGapForAlias(t *testing.T) {
	t.Parallel()
	cfg := Default()

	tests := []struct {
		alias string
		want  int
	}{
		{"fast", cfg.InitialGapMs["fast"]},
		{"deep", cfg.InitialGapMs["deep"]},
		{"nonexistent", 1200}, // default
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			t.Parallel()
			got := cfg.InitialGapForAlias(tt.alias)
			if got != tt.want {
				t.Errorf("InitialGapForAlias(%q)=%d, want %d", tt.alias, got, tt.want)
			}
		})
	}
}
