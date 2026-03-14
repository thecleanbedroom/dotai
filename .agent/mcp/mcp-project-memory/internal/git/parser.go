// Package git implements domain.GitParser by shelling out to `git`.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// Parser implements domain.GitParser using subprocess git calls.
type Parser struct {
	repoDir string
}

// NewParser creates a Git parser for the given repository directory.
func NewParser(repoDir string) *Parser {
	return &Parser{repoDir: repoDir}
}

// GetAllHashes returns all commit hashes (newest first). If limit > 0, caps results.
func (p *Parser) GetAllHashes(limit int) ([]string, error) {
	args := []string{"log", "--format=%H"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	out, err := p.run(args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var hashes []string
	for _, l := range lines {
		if h := strings.TrimSpace(l); h != "" {
			hashes = append(hashes, h)
		}
	}
	return hashes, nil
}

// GetCurrentHash returns the current HEAD commit hash.
func (p *Parser) GetCurrentHash() (string, error) {
	out, err := p.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetCommitsByHashes fetches detailed commit info using NUL-delimited format.
func (p *Parser) GetCommitsByHashes(hashes []string) ([]*domain.ParsedCommit, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	var commits []*domain.ParsedCommit

	for _, hash := range hashes {
		c, err := p.parseOneCommit(hash)
		if err != nil {
			continue // skip unparseable commits
		}
		commits = append(commits, c)
	}

	return commits, nil
}

func (p *Parser) parseOneCommit(hash string) (*domain.ParsedCommit, error) {
	// Get commit metadata
	out, err := p.run("log", "-1", "--format=%H%n%an%n%aI%n%s%n%b%n---TRAILERS---%n%(trailers:key,separator=%n)", hash)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitN(out, "\n", 5)
	if len(lines) < 4 {
		return nil, fmt.Errorf("unexpected format for %s", hash)
	}

	commit := &domain.ParsedCommit{
		Hash:    strings.TrimSpace(lines[0]),
		Author:  strings.TrimSpace(lines[1]),
		Date:    strings.TrimSpace(lines[2]),
		Message: strings.TrimSpace(lines[3]),
	}

	// Parse body and trailers
	if len(lines) > 4 {
		rest := lines[4]
		if idx := strings.Index(rest, "---TRAILERS---"); idx >= 0 {
			commit.Body = strings.TrimSpace(rest[:idx])
			trailerSection := strings.TrimSpace(rest[idx+len("---TRAILERS---"):])
			commit.Trailers = parseTrailers(trailerSection)
		} else {
			commit.Body = strings.TrimSpace(rest)
		}
	}

	// Get diff
	diff, err := p.run("diff-tree", "-p", "--no-commit-id", hash)
	if err == nil {
		commit.Diff = filterBinaryDiffs(strings.TrimSpace(diff))
	}

	// Get file list
	filesOut, err := p.run("diff-tree", "--no-commit-id", "-r", "--name-only", hash)
	if err == nil {
		for _, f := range strings.Split(strings.TrimSpace(filesOut), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				commit.Files = append(commit.Files, f)
			}
		}
	}

	return commit, nil
}

func parseTrailers(section string) map[string]string {
	trailers := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			trailers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return trailers
}

func (p *Parser) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = p.repoDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, stderr.String())
	}
	return stdout.String(), nil
}

// filterBinaryDiffs removes binary diff sections (DRY — shared utility).
func filterBinaryDiffs(diff string) string {
	if diff == "" {
		return ""
	}
	var kept []string
	current := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if current != "" && !strings.Contains(current, "Binary files") {
				kept = append(kept, current)
			}
			current = line + "\n"
		} else {
			current += line + "\n"
		}
	}
	if current != "" && !strings.Contains(current, "Binary files") {
		kept = append(kept, current)
	}
	return strings.Join(kept, "")
}
