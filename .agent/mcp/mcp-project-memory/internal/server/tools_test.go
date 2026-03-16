package server_test

import (
	"encoding/json"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestToolHandler_SearchByFile_WithData(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Insert a test memory via MemWriter
	s.MemWriter.Create(&domain.Memory{
		ID: "file-1", Summary: "File handling change", Type: "decision",
		FilePaths: []string{"internal/storage/sqlite.go"},
		SourceCommits: []string{"abc"}, Tags: []string{"storage"},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true, Importance: 50,
	})

	// Verify it's in the DB
	mem, _ := s.MemReader.Get("file-1")
	if mem == nil {
		t.Fatal("memory not found after Create")
	}

	// Now do a search by path
	results, err := s.MemReader.QueryByFile("internal/storage/sqlite.go", 20, 0)
	if err != nil {
		t.Fatalf("QueryByFile: %v", err)
	}
	if len(results) < 1 {
		t.Errorf("expected ≥1 result, got %d", len(results))
	}
}

func TestToolHandler_SearchByTopic_WithData(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	s.MemWriter.Create(&domain.Memory{
		ID: "topic-1", Summary: "Database migration decision", Type: "decision",
		FilePaths: []string{"db.go"}, SourceCommits: []string{"abc"},
		Tags: []string{"database", "migration"},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true, Importance: 70,
	})

	results, err := s.Searcher.Search("database migration", domain.SearchOpts{Limit: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// FTS search should find the memory
	if len(results) < 1 {
		t.Errorf("expected ≥1 result, got %d", len(results))
	}
}

func TestToolHandler_SearchByTopic_NoResults(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	results, err := s.Searcher.Search("xyznonexistent", domain.SearchOpts{Limit: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", len(results))
	}
}

func TestToolHandler_SearchByTopic_WithTypeFilter(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	s.MemWriter.Create(&domain.Memory{
		ID: "type-1", Summary: "A pattern for caching", Type: "pattern",
		FilePaths: []string{"cache.go"}, SourceCommits: []string{"abc"},
		Tags: []string{"cache"}, CreatedAt: "2025-01-01T00:00:00Z",
		Active: true, Importance: 60,
	})
	s.MemWriter.Create(&domain.Memory{
		ID: "type-2", Summary: "Decision about caching", Type: "decision",
		FilePaths: []string{"cache.go"}, SourceCommits: []string{"def"},
		Tags: []string{"cache"}, CreatedAt: "2025-01-01T00:00:00Z",
		Active: true, Importance: 60,
	})

	results, err := s.Searcher.Search("caching", domain.SearchOpts{
		Type:  "pattern",
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, r := range results {
		if r.Type != "pattern" {
			t.Errorf("expected type 'pattern', got %q", r.Type)
		}
	}
}

func TestToolHandler_RecallMemory_Found(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	s.MemWriter.Create(&domain.Memory{
		ID: "recall-1", Summary: "Recall test", Type: "decision",
		FilePaths: []string{"a.go"}, SourceCommits: []string{"abc"},
		Tags: []string{"test"}, CreatedAt: "2025-01-01T00:00:00Z",
		Active: true, Importance: 50,
	})

	mem, err := s.MemReader.Get("recall-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mem == nil {
		t.Fatal("expected memory recall-1")
	}
	if mem.Summary != "Recall test" {
		t.Errorf("summary: got %q", mem.Summary)
	}
}

func TestToolHandler_RecallMemory_NotFound(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	mem, err := s.MemReader.Get("00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mem != nil {
		t.Error("expected nil for non-existent memory")
	}
}

func TestToolHandler_Overview_WithMemories(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	s.MemWriter.Create(&domain.Memory{
		ID: "ov-a", Summary: "Overview test A", Type: "decision",
		FilePaths: []string{"a.go"}, SourceCommits: []string{"abc"},
		Tags: []string{"test"}, CreatedAt: "2025-01-01T00:00:00Z",
		Active: true, Importance: 50,
	})
	s.MemWriter.Create(&domain.Memory{
		ID: "ov-b", Summary: "Overview test B", Type: "pattern",
		FilePaths: []string{"b.go"}, SourceCommits: []string{"def"},
		Tags: []string{"test"}, CreatedAt: "2025-01-01T00:00:00Z",
		Active: true, Importance: 70,
	})

	stats, err := s.MemReader.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats["total_memories"] == nil {
		t.Error("expected total_memories in stats")
	}
	total := stats["total_memories"].(int)
	if total < 2 {
		t.Errorf("expected ≥2 total_memories, got %d", total)
	}
}

func TestToolHandler_Inspect_Stats(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result := s.Inspector.Inspect("stats")
	if result == "" {
		t.Error("expected non-empty inspect result for 'stats'")
	}

	// Should be valid JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Errorf("inspect stats should return JSON: %v", err)
	}
}

func TestToolHandler_Inspect_Help(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result := s.Inspector.Inspect("help")
	if result == "" {
		t.Error("expected non-empty inspect result for 'help'")
	}
}

func TestToolHandler_Inspect_FTS(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result := s.Inspector.Inspect("fts")
	if result == "" {
		t.Error("expected non-empty inspect result for 'fts'")
	}
}

func TestToolHandler_Inspect_Tables(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	result := s.Inspector.Inspect("tables")
	if result == "" {
		t.Error("expected non-empty inspect result for 'tables'")
	}
}

func TestRegisterTools_AllToolsRegistered(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.1-test")
	server.RegisterTools(mcpSrv, s)
	// Should not panic; tools registered successfully
}
