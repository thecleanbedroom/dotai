package storage_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func setupTestStore(t *testing.T) (*storage.MemoryStore, *storage.LinkStore, *storage.Database) {
	t.Helper()
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	link := storage.NewLinkStore(db)
	mem := storage.NewMemoryStore(db, link)
	return mem, link, db
}

func testMemory(id, summary, memType string) *domain.Memory {
	return &domain.Memory{
		ID:            id,
		Summary:       summary,
		Type:          memType,
		Confidence:    50,
		Importance:    75,
		SourceCommits: []string{"abc123"},
		FilePaths:     []string{"foo.go"},
		Tags:          []string{"test"},
		CreatedAt:     "2025-01-01T00:00:00Z",
		Active:        true,
	}
}

func TestMemoryStore_CreateAndGet(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m := testMemory("test-id-1", "Test memory", "decision")
	if err := mem.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := mem.Get("test-id-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected memory, got nil")
	}
	if got.Summary != "Test memory" {
		t.Errorf("expected 'Test memory', got %q", got.Summary)
	}
	if got.Type != "decision" {
		t.Errorf("expected 'decision', got %q", got.Type)
	}
}

func TestMemoryStore_GetNonExistent(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	got, err := mem.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m := testMemory("update-id", "Original", "pattern")
	mem.Create(m)

	m.Summary = "Updated"
	m.Importance = 90
	if err := mem.Update(m); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := mem.Get("update-id")
	if got.Summary != "Updated" {
		t.Errorf("expected 'Updated', got %q", got.Summary)
	}
	if got.Importance != 90 {
		t.Errorf("expected importance 90, got %d", got.Importance)
	}
}

func TestMemoryStore_Deactivate(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m := testMemory("deactivate-id", "To deactivate", "context")
	mem.Create(m)
	mem.Deactivate("deactivate-id")

	// ListAll active should not include it
	active, _ := mem.ListAll(true, 100)
	for _, a := range active {
		if a.ID == "deactivate-id" {
			t.Error("deactivated memory should not appear in active list")
		}
	}
}

func TestMemoryStore_QueryByFile(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m1 := testMemory("file-1", "Memory about foo", "decision")
	m1.FilePaths = []string{"src/foo.go", "src/bar.go"}
	mem.Create(m1)

	m2 := testMemory("file-2", "Memory about baz", "pattern")
	m2.FilePaths = []string{"src/baz.go"}
	mem.Create(m2)

	results, err := mem.QueryByFile("src/foo", 10, 0)
	if err != nil {
		t.Fatalf("QueryByFile: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestMemoryStore_Search(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m1 := testMemory("search-1", "Authentication middleware pattern", "pattern")
	m1.Tags = []string{"auth", "middleware"}
	mem.Create(m1)

	m2 := testMemory("search-2", "Database connection pooling", "convention")
	m2.Tags = []string{"database", "performance"}
	mem.Create(m2)

	// Search for "auth"
	results, err := mem.Search("auth", domain.SearchOpts{Match: "any", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'auth', got %d", len(results))
	}

	// Search with type filter
	results, err = mem.Search("middleware", domain.SearchOpts{Match: "any", Type: "decision", Limit: 10})
	if err != nil {
		t.Fatalf("Search with type: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for 'middleware' type=decision, got %d", len(results))
	}
}

func TestMemoryStore_Stats(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	mem.Create(testMemory("stats-1", "Decision one", "decision"))
	mem.Create(testMemory("stats-2", "Pattern one", "pattern"))

	stats, err := mem.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	total, ok := stats["total_memories"].(int)
	if !ok || total != 2 {
		t.Errorf("expected total_memories=2, got %v", stats["total_memories"])
	}
}

func TestLinkStore_CreateAndGet(t *testing.T) {
	_, link, db := setupTestStore(t)
	mem := storage.NewMemoryStore(db, link)
	defer db.Close()

	mem.Create(testMemory("link-a", "Memory A", "decision"))
	mem.Create(testMemory("link-b", "Memory B", "pattern"))

	l := &domain.MemoryLink{
		MemoryIDA:    "link-a",
		MemoryIDB:    "link-b",
		Relationship: "supports",
		Strength:     0.8,
	}
	if err := link.CreateLink(l); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if l.ID == 0 {
		t.Error("expected link ID to be set")
	}

	links, err := link.GetLinksFor("link-a")
	if err != nil {
		t.Fatalf("GetLinksFor: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}

	// Bidirectional
	links, err = link.GetLinksFor("link-b")
	if err != nil {
		t.Fatalf("GetLinksFor: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("expected 1 bidirectional link, got %d", len(links))
	}
}

func TestLinkStore_GetLinkedIDs(t *testing.T) {
	_, link, db := setupTestStore(t)
	mem := storage.NewMemoryStore(db, link)
	defer db.Close()

	mem.Create(testMemory("lid-a", "A", "decision"))
	mem.Create(testMemory("lid-b", "B", "pattern"))
	link.CreateLink(&domain.MemoryLink{
		MemoryIDA: "lid-a", MemoryIDB: "lid-b",
		Relationship: "related_to", Strength: 0.5,
	})

	ids, _ := link.GetLinkedIDs("lid-a")
	if len(ids) != 1 || ids[0] != "lid-b" {
		t.Errorf("expected [lid-b], got %v", ids)
	}
}

func TestBuildMetaStore(t *testing.T) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	bm := storage.NewBuildMetaStore(db)

	// Initially empty
	last, err := bm.GetLast()
	if err != nil {
		t.Fatalf("GetLast: %v", err)
	}
	if last != nil {
		t.Error("expected nil last build")
	}

	// Record
	entry := &domain.BuildMetaEntry{
		BuildType:   "incremental",
		CommitCount: 5,
		MemoryCount: 10,
	}
	if err := bm.Record(entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if entry.ID == 0 {
		t.Error("expected ID to be set")
	}

	last, _ = bm.GetLast()
	if last == nil {
		t.Fatal("expected last build")
	}
	if last.CommitCount != 5 {
		t.Errorf("expected commit_count=5, got %d", last.CommitCount)
	}
}
