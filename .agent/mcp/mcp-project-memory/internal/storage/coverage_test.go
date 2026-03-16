package storage_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

// Tests for previously uncovered methods in memorystore, linkstore, and sqlite.

func TestMemoryStore_Touch(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m := testMemory("touch-id", "Touch test", "context")
	mem.Create(m)

	for i := 0; i < 3; i++ {
		if err := mem.Touch("touch-id"); err != nil {
			t.Fatalf("Touch: %v", err)
		}
	}

	got, _ := mem.Get("touch-id")
	if got.AccessCount != 3 {
		t.Errorf("expected access_count=3, got %d", got.AccessCount)
	}
	if got.AccessedAt == "" {
		t.Error("expected accessed_at to be set")
	}
}

func TestMemoryStore_GetMany(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	mem.Create(testMemory("many-1", "One", "decision"))
	mem.Create(testMemory("many-2", "Two", "pattern"))
	mem.Create(testMemory("many-3", "Three", "context"))

	results, err := mem.GetMany([]string{"many-1", "many-3"})
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestMemoryStore_GetMany_Empty(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	results, err := mem.GetMany(nil)
	if err != nil {
		t.Fatalf("GetMany nil: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestMemoryStore_Count(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	mem.Create(testMemory("cnt-1", "One", "decision"))
	mem.Create(testMemory("cnt-2", "Two", "pattern"))

	cnt, err := mem.Count(true)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 2 {
		t.Errorf("expected 2 active, got %d", cnt)
	}

	// Deactivate one
	mem.Deactivate("cnt-1")
	cnt, _ = mem.Count(true)
	if cnt != 1 {
		t.Errorf("expected 1 active, got %d", cnt)
	}

	// Count all
	cnt, _ = mem.Count(false)
	if cnt != 2 {
		t.Errorf("expected 2 total, got %d", cnt)
	}
}

func TestMemoryStore_ListAll_Active(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m1 := testMemory("la-1", "Active", "decision")
	m1.Importance = 90
	mem.Create(m1)

	m2 := testMemory("la-2", "Inactive", "pattern")
	m2.Importance = 50
	mem.Create(m2)
	mem.Deactivate("la-2")

	// Active only
	active, _ := mem.ListAll(true, 100)
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}

	// All
	all, _ := mem.ListAll(false, 100)
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestMemoryStore_Search_Match_All(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m := testMemory("match-all", "Database authentication middleware", "pattern")
	m.Tags = []string{"database", "auth", "middleware"}
	mem.Create(m)

	// Match "all" requires all terms
	results, _ := mem.Search("database auth", domain.SearchOpts{Match: "all", Limit: 10})
	if len(results) != 1 {
		t.Errorf("match=all 'database auth': expected 1, got %d", len(results))
	}

	// Match "all" with a missing term
	results, _ = mem.Search("database unicorn", domain.SearchOpts{Match: "all", Limit: 10})
	if len(results) != 0 {
		t.Errorf("match=all 'database unicorn': expected 0, got %d", len(results))
	}
}

func TestMemoryStore_Search_ExcludeTags(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m1 := testMemory("excl-1", "Some decision about caching", "decision")
	m1.Tags = []string{"caching", "skip-me"}
	mem.Create(m1)

	m2 := testMemory("excl-2", "Another caching decision", "decision")
	m2.Tags = []string{"caching"}
	mem.Create(m2)

	results, _ := mem.Search("caching", domain.SearchOpts{
		Match: "any", Limit: 10,
		ExcludeTags: []string{"skip-me"},
	})
	if len(results) != 1 {
		t.Errorf("expected 1 result after exclude, got %d", len(results))
	}
}

func TestLinkStore_DeleteForMemory(t *testing.T) {
	_, link, db := setupTestStore(t)
	mem := storage.NewMemoryStore(db, link)
	defer db.Close()

	mem.Create(testMemory("del-a", "A", "decision"))
	mem.Create(testMemory("del-b", "B", "pattern"))
	link.CreateLink(&domain.MemoryLink{
		MemoryIDA: "del-a", MemoryIDB: "del-b",
		Relationship: "related_to", Strength: 0.5,
	})

	if err := link.DeleteForMemory("del-a"); err != nil {
		t.Fatalf("DeleteForMemory: %v", err)
	}

	links, _ := link.GetLinksFor("del-a")
	if len(links) != 0 {
		t.Errorf("expected 0 links after delete, got %d", len(links))
	}
}

func TestLinkStore_ListAll(t *testing.T) {
	_, link, db := setupTestStore(t)
	mem := storage.NewMemoryStore(db, link)
	defer db.Close()

	mem.Create(testMemory("list-a", "A", "decision"))
	mem.Create(testMemory("list-b", "B", "pattern"))
	mem.Create(testMemory("list-c", "C", "context"))
	link.CreateLink(&domain.MemoryLink{
		MemoryIDA: "list-a", MemoryIDB: "list-b",
		Relationship: "supports", Strength: 0.7,
	})
	link.CreateLink(&domain.MemoryLink{
		MemoryIDA: "list-b", MemoryIDB: "list-c",
		Relationship: "related_to", Strength: 0.5,
	})

	all, err := link.ListAll(100)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 links, got %d", len(all))
	}
}



func TestMemoryStore_Search_SinceUntil(t *testing.T) {
	mem, _, db := setupTestStore(t)
	defer db.Close()

	m1 := testMemory("time-1", "Old memory", "decision")
	m1.CreatedAt = "2024-01-01T00:00:00Z"
	mem.Create(m1)

	m2 := testMemory("time-2", "New memory", "decision")
	m2.CreatedAt = "2025-06-01T00:00:00Z"
	mem.Create(m2)

	// Since filter
	results, _ := mem.Search("memory", domain.SearchOpts{
		Match: "any", Limit: 10,
		Since: "2025-01-01",
	})
	if len(results) != 1 {
		t.Errorf("since filter: expected 1, got %d", len(results))
	}

	// Until filter
	results, _ = mem.Search("memory", domain.SearchOpts{
		Match: "any", Limit: 10,
		Until: "2024-12-31",
	})
	if len(results) != 1 {
		t.Errorf("until filter: expected 1, got %d", len(results))
	}
}
