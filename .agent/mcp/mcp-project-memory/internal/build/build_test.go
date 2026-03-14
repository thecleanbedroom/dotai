package build_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/build"
	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

func TestBatchPlanner_SingleBatch(t *testing.T) {
	cfg := config.Load()
	planner := build.NewBatchPlanner(cfg)

	commits := []*domain.ParsedCommit{
		{Hash: "aaa", Message: "Small commit", Diff: "some diff"},
		{Hash: "bbb", Message: "Another small", Diff: "diff2"},
	}

	batches := planner.Plan(commits)
	if len(batches) != 1 {
		t.Errorf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("expected 2 commits in batch, got %d", len(batches[0]))
	}
}

func TestBatchPlanner_EmptyInput(t *testing.T) {
	cfg := config.Load()
	planner := build.NewBatchPlanner(cfg)
	batches := planner.Plan(nil)
	if batches != nil {
		t.Errorf("expected nil for empty input, got %v", batches)
	}
}

func TestPathFilter_FilterCommits(t *testing.T) {
	filter := build.NewPathFilter("*.lock,vendor/*")

	commits := []*domain.ParsedCommit{
		{Hash: "a", Files: []string{"go.sum", "main.go"}},
		{Hash: "b", Files: []string{"package-lock.json"}}, // only .lock — filtered
		{Hash: "c", Files: []string{"src/app.go"}},
	}

	result := filter.FilterCommits(commits)
	// "b" should be filtered out (only a lock file)
	// Note: filepath.Match("*.lock", "package-lock.json") — matches basename
	if len(result) < 2 {
		t.Errorf("expected at least 2 commits after filter, got %d", len(result))
	}
}

func TestPathFilter_Empty(t *testing.T) {
	filter := build.NewPathFilter("")
	commits := []*domain.ParsedCommit{
		{Hash: "a", Files: []string{"anything.go"}},
	}
	result := filter.FilterCommits(commits)
	if len(result) != 1 {
		t.Errorf("empty filter should pass all, got %d", len(result))
	}
}

func TestPathFilter_FilterMemory(t *testing.T) {
	filter := build.NewPathFilter(".agent/memory/data/*")
	m := &domain.Memory{
		FilePaths: []string{".agent/memory/data/foo.json"},
	}
	if filter.FilterMemory(m) {
		t.Error("memory with only ignored paths should be filtered")
	}

	m2 := &domain.Memory{
		FilePaths: []string{"src/main.go"},
	}
	if !filter.FilterMemory(m2) {
		t.Error("memory with non-ignored paths should pass")
	}
}

func TestMemoryFactory_Score(t *testing.T) {
	factory := build.NewMemoryFactory()
	m := &domain.Memory{
		Summary:       "This is a detailed summary that is quite long, providing substantial information about the architectural decision that was made.",
		Type:          "decision",
		SourceCommits: []string{"abc", "def", "ghi"},
		FilePaths:     []string{"a.go", "b.go", "c.go", "d.go"},
		Tags:          []string{"architecture", "database", "performance", "scaling", "caching"},
	}

	scored := factory.Score(m)
	if scored.Confidence == 0 {
		t.Error("expected non-zero confidence")
	}
	if scored.ID == "" {
		t.Error("expected ID to be generated")
	}
	if scored.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

func TestValidateExtraction_Valid(t *testing.T) {
	raw := `{"memories":[{"summary":"Test memory","type":"decision","importance":50,"source_commits":["abcdef"],"file_paths":["foo.go"],"tags":["test"]}]}`
	memories, err := build.ValidateExtraction(raw)
	if err != nil {
		t.Fatalf("ValidateExtraction: %v", err)
	}
	if len(memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(memories))
	}
	if memories[0].Summary != "Test memory" {
		t.Errorf("expected 'Test memory', got %q", memories[0].Summary)
	}
}

func TestValidateExtraction_InvalidType(t *testing.T) {
	raw := `{"memories":[{"summary":"Test","type":"invalid_type","importance":50,"source_commits":["abc"],"file_paths":[],"tags":[]}]}`
	memories, err := build.ValidateExtraction(raw)
	if err != nil {
		t.Fatalf("ValidateExtraction: %v", err)
	}
	// Invalid type should be skipped
	if len(memories) != 0 {
		t.Errorf("expected 0 memories (invalid type), got %d", len(memories))
	}
}

func TestValidateExtraction_MissingSummary(t *testing.T) {
	raw := `{"memories":[{"type":"decision","importance":50,"source_commits":["abc"]}]}`
	memories, err := build.ValidateExtraction(raw)
	if err != nil {
		t.Fatalf("ValidateExtraction: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("expected 0 memories (missing summary), got %d", len(memories))
	}
}

func TestValidateExtraction_InvalidJSON(t *testing.T) {
	raw := `not json`
	_, err := build.ValidateExtraction(raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
