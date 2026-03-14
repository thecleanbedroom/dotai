package build

import (
	"path/filepath"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// PathFilter excludes files and memories by glob patterns.
type PathFilter struct {
	patterns []string
}

// NewPathFilter creates a filter from a comma-separated glob string.
func NewPathFilter(ignoreSpec string) *PathFilter {
	var patterns []string
	for _, p := range strings.Split(ignoreSpec, ",") {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}
	return &PathFilter{patterns: patterns}
}

// FilterCommits removes commits where ALL changed files match ignore patterns.
func (pf *PathFilter) FilterCommits(commits []*domain.ParsedCommit) []*domain.ParsedCommit {
	if len(pf.patterns) == 0 {
		return commits
	}
	var result []*domain.ParsedCommit
	for _, c := range commits {
		if pf.hasNonIgnoredFiles(c.Files) {
			result = append(result, c)
		}
	}
	return result
}

// FilterMemory returns true if the memory should be kept (has non-ignored paths).
func (pf *PathFilter) FilterMemory(m *domain.Memory) bool {
	if len(pf.patterns) == 0 || len(m.FilePaths) == 0 {
		return true
	}
	return pf.hasNonIgnoredFiles(m.FilePaths)
}

func (pf *PathFilter) hasNonIgnoredFiles(files []string) bool {
	for _, f := range files {
		if !pf.matchesAny(f) {
			return true // at least one file not ignored
		}
	}
	return len(files) == 0 // keep if no files at all
}

func (pf *PathFilter) matchesAny(path string) bool {
	for _, pattern := range pf.patterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also try matching the basename
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}
