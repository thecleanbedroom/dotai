package domain

import (
	"testing"
	"time"
)

func TestFormatTime(t *testing.T) {
	t.Parallel()

	ts := float64(time.Date(2026, 3, 14, 10, 30, 15, 0, time.UTC).Unix())

	result := FormatTime(ts)
	if result == "" {
		t.Error("FormatTime returned empty string")
	}
}

func TestFormatTimeShort(t *testing.T) {
	t.Parallel()

	ts := float64(time.Date(2026, 3, 14, 10, 30, 15, 0, time.UTC).Unix())

	result := FormatTimeShort(ts)
	if result == "" {
		t.Error("FormatTimeShort returned empty string")
	}
}
