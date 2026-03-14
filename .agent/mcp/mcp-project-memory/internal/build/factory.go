package build

import (
	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/google/uuid"
)

// MemoryFactory creates and scores Memory objects from LLM output.
type MemoryFactory struct{}

// NewMemoryFactory creates a factory.
func NewMemoryFactory() *MemoryFactory { return &MemoryFactory{} }

// Score applies confidence scoring to a memory based on thresholds.
func (f *MemoryFactory) Score(m *domain.Memory) *domain.Memory {
	score := 0

	// Source commits
	for threshold, points := range config.ConfidenceCommitsThresholds() {
		if len(m.SourceCommits) >= threshold {
			score = max(score, points)
		}
	}

	// File paths
	for threshold, points := range config.ConfidenceFilesThresholds() {
		if len(m.FilePaths) >= threshold {
			score = max(score, points)
		}
	}

	// Summary length
	for threshold, points := range config.ConfidenceSummaryThresholds() {
		if len(m.Summary) >= threshold {
			score = max(score, points)
		}
	}

	// Tags
	for threshold, points := range config.ConfidenceTagsThresholds() {
		if len(m.Tags) >= threshold {
			score = max(score, points)
		}
	}

	m.Confidence = score

	// Ensure ID
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	// Ensure created_at
	if m.CreatedAt == "" {
		m.CreatedAt = domain.NowUTC()
	}

	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
