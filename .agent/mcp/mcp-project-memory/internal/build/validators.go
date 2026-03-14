package build

import (
	"encoding/json"
	"fmt"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/google/uuid"
)

// ValidateExtraction validates and parses LLM extraction output into memories.
func ValidateExtraction(raw string) ([]*domain.Memory, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("parse extraction JSON: %w", err)
	}

	memoriesRaw, ok := data["memories"].([]any)
	if !ok {
		return nil, fmt.Errorf("extraction output missing 'memories' array")
	}

	var memories []*domain.Memory
	for i, item := range memoriesRaw {
		mMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		m, err := validateMemoryDict(mMap, i)
		if err != nil {
			fmt.Printf("  warning: skipping memory %d: %v\n", i, err)
			continue
		}
		memories = append(memories, m)
	}

	return memories, nil
}

// ValidateTriage validates triage LLM output.
func ValidateTriage(raw string) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("parse triage JSON: %w", err)
	}
	if _, ok := data["decisions"].([]any); !ok {
		return nil, fmt.Errorf("triage output missing 'decisions' array")
	}
	return data, nil
}

// ValidateLinking validates linking LLM output.
func ValidateLinking(raw string) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("parse linking JSON: %w", err)
	}
	if _, ok := data["links"].([]any); !ok {
		return nil, fmt.Errorf("linking output missing 'links' array")
	}
	return data, nil
}

// validateMemoryDict validates a single memory dictionary from LLM output.
// Shared field checks (DRY) used by all validators.
func validateMemoryDict(m map[string]any, idx int) (*domain.Memory, error) {
	// Required fields
	summary, _ := m["summary"].(string)
	if summary == "" {
		return nil, fmt.Errorf("memory %d: missing summary", idx)
	}

	memType, _ := m["type"].(string)
	if !domain.MemoryTypes[memType] {
		return nil, fmt.Errorf("memory %d: invalid type %q", idx, memType)
	}

	// Importance validation
	importance := 50
	if v, ok := m["importance"].(float64); ok {
		importance = int(v)
	}
	if importance < 0 || importance > config.ValidationMaxImportance() {
		return nil, fmt.Errorf("memory %d: importance %d out of range", idx, importance)
	}

	// Source commits validation
	var sourceCommits []string
	if sc, ok := m["source_commits"].([]any); ok {
		for _, c := range sc {
			if s, ok := c.(string); ok {
				if len(s) >= config.ValidationMinCommitHashLength() {
					sourceCommits = append(sourceCommits, s)
				}
			}
		}
	}

	// File paths
	var filePaths []string
	if fp, ok := m["file_paths"].([]any); ok {
		for _, f := range fp {
			if s, ok := f.(string); ok {
				filePaths = append(filePaths, s)
			}
		}
	}

	// Tags
	var tags []string
	if t, ok := m["tags"].([]any); ok {
		for _, tag := range t {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	mem := &domain.Memory{
		ID:            uuid.New().String(),
		Summary:       summary,
		Type:          memType,
		Importance:    importance,
		SourceCommits: sourceCommits,
		FilePaths:     filePaths,
		Tags:          tags,
		CreatedAt:     domain.NowUTC(),
		Active:        true,
	}

	if mem.SourceCommits == nil {
		mem.SourceCommits = []string{}
	}
	if mem.FilePaths == nil {
		mem.FilePaths = []string{}
	}
	if mem.Tags == nil {
		mem.Tags = []string{}
	}

	return mem, nil
}
