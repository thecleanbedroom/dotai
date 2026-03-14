package build

import (
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
func (bp *BatchPlanner) Plan(commits []*domain.ParsedCommit) [][]*domain.ParsedCommit {
	if len(commits) == 0 {
		return nil
	}

	var batches [][]*domain.ParsedCommit
	var current []*domain.ParsedCommit
	currentTokens := 0

	for _, c := range commits {
		tokens := estimateCommitTokens(c)

		// If single commit exceeds budget, put it alone
		if tokens >= bp.tokenBudget {
			if len(current) > 0 {
				batches = append(batches, current)
				current = nil
				currentTokens = 0
			}
			batches = append(batches, []*domain.ParsedCommit{c})
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
