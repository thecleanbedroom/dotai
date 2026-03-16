package build

import (
	"fmt"
	"os"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// BatchPlanner groups commits into batches that fit within the token budget.
type BatchPlanner struct {
	tokenBudget int
	maxCommits  int
}

// NewBatchPlanner creates a planner from settings.
func NewBatchPlanner(cfg *config.Settings) *BatchPlanner {
	return &BatchPlanner{
		tokenBudget: cfg.BatchingTokenBudget(),
		maxCommits:  cfg.BatchingMaxCommits(),
	}
}

// Plan groups commits into batches respecting token and commit limits.
// Oversized commits are split into sub-commits by file boundaries.
func (bp *BatchPlanner) Plan(commits []*domain.ParsedCommit) [][]*domain.ParsedCommit {
	if len(commits) == 0 {
		return nil
	}

	var batches [][]*domain.ParsedCommit
	var current []*domain.ParsedCommit
	currentTokens := 0

	for _, c := range commits {
		tokens := estimateCommitTokens(c)

		// If single commit exceeds budget, split it by file boundaries
		if tokens >= bp.tokenBudget {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
				currentTokens = 0
			}
			subs := splitOversizedCommit(c, bp.tokenBudget)
			for _, sc := range subs {
				batches = append(batches, []*domain.ParsedCommit{sc})
			}
			continue
		}

		// Check if adding this commit would exceed limits
		if currentTokens+tokens > bp.tokenBudget || len(current) >= bp.maxCommits {
			if len(current) > 0 {
				batches = append(batches, current)
			}
			current = nil
			currentTokens = 0
		}

		current = append(current, c)
		currentTokens += tokens
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}

	return batches
}

func estimateCommitTokens(c *domain.ParsedCommit) int {
	text := c.Message + c.Body + c.Diff
	for _, f := range c.Files {
		text += f
	}
	return config.EstimateTokens(text)
}

// splitOversizedCommit splits a commit that exceeds the token budget into
// smaller sub-commits by file boundaries (matching the Python version).
// Each sub-commit gets the same metadata but a subset of files/diffs.
// Individual file diffs that exceed the budget are truncated.
func splitOversizedCommit(c *domain.ParsedCommit, budget int) []*domain.ParsedCommit {
	charsPerToken := config.TokenCharsPerToken()

	// Calculate metadata overhead (hash, author, date, message, trailers)
	metaChars := len(c.Hash) + len(c.Author) + len(c.Date) + len(c.Message) + 120
	for k, v := range c.Trailers {
		metaChars += len(k) + len(v)
	}
	metaTokens := metaChars / charsPerToken

	available := budget - metaTokens
	if available < budget/2 {
		available = budget / 2
	}
	availableChars := available * charsPerToken

	diffByFile := splitDiffByFile(c.Diff)

	var subs []*domain.ParsedCommit
	var curFiles []string
	var curDiffs []string
	curChars := 0

	for _, f := range c.Files {
		fileDiff := diffByFile[f]
		fileChars := len(f) + len(fileDiff) + 2

		// Truncate individual file diffs that exceed the budget (last resort)
		truncChars := budget * charsPerToken
		if fileChars > truncChars {
			maxDiffChars := truncChars - len(f) - 100
			if maxDiffChars > 0 && len(fileDiff) > 0 {
				if maxDiffChars < len(fileDiff) {
					fileDiff = fileDiff[:maxDiffChars] + "\n... [diff truncated]"
				}
				fileChars = len(f) + len(fileDiff) + 2
			}
		}

		// Flush current sub-commit if adding this file would exceed budget
		if len(curFiles) > 0 && curChars+fileChars > availableChars {
			subs = append(subs, &domain.ParsedCommit{
				Hash:     c.Hash,
				Author:   c.Author,
				Date:     c.Date,
				Message:  fmt.Sprintf("%s [part %d]", c.Message, len(subs)+1),
				Diff:     strings.Join(curDiffs, "\n"),
				Files:    curFiles,
				Trailers: c.Trailers,
			})
			curFiles = nil
			curDiffs = nil
			curChars = 0
		}

		curFiles = append(curFiles, f)
		if fileDiff != "" {
			curDiffs = append(curDiffs, fileDiff)
		}
		curChars += fileChars
	}

	// Handle body text
	if c.Body != "" {
		if len(curFiles) > 0 {
			subs = append(subs, &domain.ParsedCommit{
				Hash:     c.Hash,
				Author:   c.Author,
				Date:     c.Date,
				Message:  fmt.Sprintf("%s [part %d]", c.Message, len(subs)+1),
				Diff:     strings.Join(curDiffs, "\n"),
				Files:    curFiles,
				Trailers: c.Trailers,
			})
			curFiles = nil
			curDiffs = nil
		}

		bodyBudgetChars := availableChars
		for i := 0; i < len(c.Body); i += bodyBudgetChars {
			end := i + bodyBudgetChars
			if end > len(c.Body) {
				end = len(c.Body)
			}
			trailers := c.Trailers
			if i > 0 {
				trailers = nil
			}
			subs = append(subs, &domain.ParsedCommit{
				Hash:     c.Hash,
				Author:   c.Author,
				Date:     c.Date,
				Message:  fmt.Sprintf("%s [body part %d]", c.Message, len(subs)+1),
				Body:     c.Body[i:end],
				Trailers: trailers,
			})
		}
	} else if len(curFiles) > 0 {
		subs = append(subs, &domain.ParsedCommit{
			Hash:     c.Hash,
			Author:   c.Author,
			Date:     c.Date,
			Message:  fmt.Sprintf("%s [part %d]", c.Message, len(subs)+1),
			Diff:     strings.Join(curDiffs, "\n"),
			Files:    curFiles,
			Trailers: c.Trailers,
		})
	}

	if len(subs) == 0 {
		subs = []*domain.ParsedCommit{c}
	}

	fmt.Fprintf(os.Stderr, "    split oversized commit %s into %d sub-batches\n",
		c.Hash[:min(8, len(c.Hash))], len(subs))
	return subs
}

// splitDiffByFile splits a unified diff into per-file chunks.
// Returns a map of file path → diff text for that file.
func splitDiffByFile(diff string) map[string]string {
	result := map[string]string{}
	if diff == "" {
		return result
	}

	var currentFile string
	var currentLines []string

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			// Flush previous file
			if currentFile != "" && len(currentLines) > 0 {
				result[currentFile] = strings.Join(currentLines, "\n")
			}
			// Extract file path from "diff --git a/foo b/foo"
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) > 1 {
				currentFile = parts[1]
			} else {
				currentFile = ""
			}
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if currentFile != "" && len(currentLines) > 0 {
		result[currentFile] = strings.Join(currentLines, "\n")
	}

	return result
}
