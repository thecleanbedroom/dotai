package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/git"
)

// setupGitRepo creates a temporary git repository with test commits.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", args[1], err, out)
		}
	}

	// Create 3 commits
	for i, msg := range []string{"Initial commit", "Add feature", "Fix bug"} {
		file := filepath.Join(dir, "file"+string(rune('a'+i))+".go")
		os.WriteFile(file, []byte("package main\n// "+msg+"\n"), 0o644)
		exec.Command("git", "-C", dir, "add", ".").Run()
		exec.Command("git", "-C", dir, "commit", "-m", msg).Run()
	}

	return dir
}

func TestParser_GetAllHashes(t *testing.T) {
	dir := setupGitRepo(t)
	p := git.NewParser(dir)

	hashes, err := p.GetAllHashes(0)
	if err != nil {
		t.Fatalf("GetAllHashes: %v", err)
	}
	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes, got %d", len(hashes))
	}
	// Each hash should be 40 hex chars
	for _, h := range hashes {
		if len(h) != 40 {
			t.Errorf("expected 40-char hash, got %d chars: %q", len(h), h)
		}
	}
}

func TestParser_GetAllHashes_WithLimit(t *testing.T) {
	dir := setupGitRepo(t)
	p := git.NewParser(dir)

	hashes, err := p.GetAllHashes(2)
	if err != nil {
		t.Fatalf("GetAllHashes: %v", err)
	}
	if len(hashes) != 2 {
		t.Errorf("expected 2 hashes with limit, got %d", len(hashes))
	}
}

func TestParser_GetCurrentHash(t *testing.T) {
	dir := setupGitRepo(t)
	p := git.NewParser(dir)

	hash, err := p.GetCurrentHash()
	if err != nil {
		t.Fatalf("GetCurrentHash: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d: %q", len(hash), hash)
	}
}

func TestParser_GetCommitsByHashes(t *testing.T) {
	dir := setupGitRepo(t)
	p := git.NewParser(dir)

	hashes, _ := p.GetAllHashes(0)
	commits, err := p.GetCommitsByHashes(hashes)
	if err != nil {
		t.Fatalf("GetCommitsByHashes: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("expected 3 commits, got %d", len(commits))
	}

	// Verify structure
	for _, c := range commits {
		if c.Hash == "" {
			t.Error("commit hash should not be empty")
		}
		if c.Author == "" {
			t.Error("commit author should not be empty")
		}
		if c.Message == "" {
			t.Error("commit message should not be empty")
		}
		// Note: initial commit may have no files from diff-tree
	}
}

func TestParser_GetCommitsByHashes_Empty(t *testing.T) {
	dir := setupGitRepo(t)
	p := git.NewParser(dir)

	commits, err := p.GetCommitsByHashes(nil)
	if err != nil {
		t.Fatalf("GetCommitsByHashes nil: %v", err)
	}
	if commits != nil {
		t.Errorf("expected nil, got %v", commits)
	}
}

func TestParser_CommitWithTrailers(t *testing.T) {
	dir := setupGitRepo(t)

	// Add a commit with trailers
	file := filepath.Join(dir, "trailer.go")
	os.WriteFile(file, []byte("package main\n"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	cmd := exec.Command("git", "-C", dir, "commit", "-m", "Trailer commit\n\nSigned-off-by: Test <test@test.com>\nMemory-scope: architecture")
	cmd.Run()

	p := git.NewParser(dir)
	hash, _ := p.GetCurrentHash()
	commits, _ := p.GetCommitsByHashes([]string{hash})

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.Message != "Trailer commit" {
		t.Errorf("message: got %q", c.Message)
	}
}

func TestParser_InvalidRepo(t *testing.T) {
	p := git.NewParser("/tmp/nonexistent-repo-" + t.Name())
	_, err := p.GetAllHashes(0)
	if err == nil {
		t.Error("expected error for invalid repo")
	}
}
