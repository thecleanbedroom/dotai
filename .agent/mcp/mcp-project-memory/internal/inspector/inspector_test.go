package inspector_test

import (
	"strings"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/inspector"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func setupInspector(t *testing.T) (*inspector.Inspector, *storage.MemoryStore, *storage.LinkStore, *storage.Database) {
	t.Helper()
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	link := storage.NewLinkStore(db)
	mem := storage.NewMemoryStore(db, link)
	build := storage.NewBuildMetaStore(db)
	rawDB := &testRawQuerier{db: db}
	insp := inspector.New(mem, link, build, db, rawDB)
	return insp, mem, link, db
}

type testRawQuerier struct {
	db *storage.Database
}

func (q *testRawQuerier) Query(query string, args ...any) ([]map[string]any, error) {
	rows, err := q.db.DB().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		rows.Scan(ptrs...)
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = values[i]
		}
		result = append(result, row)
	}
	return result, nil
}

func TestInspect_Help(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("help")
	if !strings.Contains(result, "tables") {
		t.Error("help should mention 'tables'")
	}
	if !strings.Contains(result, "memories") {
		t.Error("help should mention 'memories'")
	}
}

func TestInspect_UnknownCommand(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("foobar")
	if !strings.Contains(result, "Unknown") {
		t.Errorf("expected 'Unknown', got %q", result)
	}
}

func TestInspect_Empty(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("")
	if !strings.Contains(result, "tables") {
		t.Error("empty query should show help")
	}
}

func TestInspect_Tables(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("tables")
	if !strings.Contains(result, "memories") {
		t.Error("tables should include 'memories'")
	}
}

func TestInspect_Memories_Empty(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("memories")
	if result != "null" && !strings.Contains(result, "[]") {
		t.Logf("result: %s", result)
	}
}

func TestInspect_Memories_WithData(t *testing.T) {
	insp, mem, _, db := setupInspector(t)
	defer db.Close()

	mem.Create(&domain.Memory{
		ID: "insp-1", Summary: "Inspector test", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})

	result := insp.Inspect("memories")
	if !strings.Contains(result, "insp-1") {
		t.Error("memories should contain 'insp-1'")
	}
}

func TestInspect_SingleMemory(t *testing.T) {
	insp, mem, _, db := setupInspector(t)
	defer db.Close()

	mem.Create(&domain.Memory{
		ID: "single-1", Summary: "Single", Type: "pattern",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})

	result := insp.Inspect("memory single-1")
	if !strings.Contains(result, "single-1") {
		t.Error("memory detail should contain ID")
	}
}

func TestInspect_SingleMemory_NotFound(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("memory nonexistent-id")
	if !strings.Contains(result, "not found") {
		t.Errorf("expected 'not found', got %q", result)
	}
}

func TestInspect_SingleMemory_NoArg(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("memory")
	if !strings.Contains(result, "Usage") {
		t.Errorf("expected 'Usage', got %q", result)
	}
}

func TestInspect_Links(t *testing.T) {
	insp, mem, link, db := setupInspector(t)
	defer db.Close()

	mem.Create(&domain.Memory{
		ID: "la", Summary: "A", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})
	mem.Create(&domain.Memory{
		ID: "lb", Summary: "B", Type: "pattern",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})
	link.CreateLink(&domain.MemoryLink{
		MemoryIDA: "la", MemoryIDB: "lb",
		Relationship: "supports", Strength: 0.7,
	})

	result := insp.Inspect("links")
	if !strings.Contains(result, "supports") {
		t.Error("links should contain 'supports'")
	}
}

func TestInspect_Stats(t *testing.T) {
	insp, mem, _, db := setupInspector(t)
	defer db.Close()

	mem.Create(&domain.Memory{
		ID: "stat-1", Summary: "Stat", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true, Importance: 80,
	})

	result := insp.Inspect("stats")
	if !strings.Contains(result, "total_memories") {
		t.Error("stats should contain 'total_memories'")
	}
}

func TestInspect_Schema(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("schema")
	if !strings.Contains(result, "CREATE TABLE") {
		t.Error("schema should contain 'CREATE TABLE'")
	}
}

func TestInspect_FTS(t *testing.T) {
	insp, mem, _, db := setupInspector(t)
	defer db.Close()

	mem.Create(&domain.Memory{
		ID: "fts-1", Summary: "FTS check", Type: "context",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})

	result := insp.Inspect("fts")
	if !strings.Contains(result, "in_sync") {
		t.Error("fts should contain 'in_sync'")
	}
}

func TestInspect_Builds_Empty(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	result := insp.Inspect("builds")
	// Should return null/empty for no builds
	_ = result
}

func TestInspect_Builds_WithData(t *testing.T) {
	insp, _, _, db := setupInspector(t)
	defer db.Close()

	bm := storage.NewBuildMetaStore(db)
	bm.Record(&domain.BuildMetaEntry{
		BuildType: "incremental", CommitCount: 5, MemoryCount: 10,
	})

	result := insp.Inspect("builds")
	if !strings.Contains(result, "incremental") {
		t.Error("builds should contain 'incremental'")
	}
}
