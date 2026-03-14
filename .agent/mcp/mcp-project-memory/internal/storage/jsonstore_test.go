package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

func TestJSONStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()

	js := storage.NewJSONStore()
	m := &domain.Memory{
		ID:            "json-1",
		Summary:       "JSON test",
		Type:          "decision",
		Confidence:    50,
		Importance:    75,
		SourceCommits: []string{"abc"},
		FilePaths:     []string{"foo.go"},
		Tags:          []string{"test"},
		CreatedAt:     "2025-01-01T00:00:00Z",
		Active:        true,
	}

	if err := js.Write(m, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// File should exist
	path := filepath.Join(dir, "memories", "json-1.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected JSON file to exist")
	}

	// Read it back
	got, err := js.Read("json-1", dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got == nil {
		t.Fatal("expected memory, got nil")
	}
	if got.Summary != "JSON test" {
		t.Errorf("expected 'JSON test', got %q", got.Summary)
	}
}

func TestJSONStore_ReadNonExistent(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	got, err := js.Read("nonexistent", dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestJSONStore_ReadAll(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	for i, id := range []string{"a-1", "b-2", "c-3"} {
		js.Write(&domain.Memory{
			ID: id, Summary: id, Type: "decision",
			SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
			CreatedAt: "2025-01-01T00:00:00Z", Active: i < 2, // c-3 inactive
		}, dir)
	}

	memories, err := js.ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	// Only active memories
	if len(memories) != 2 {
		t.Errorf("expected 2 active memories, got %d", len(memories))
	}
}

func TestJSONStore_ReadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	memories, err := js.ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("expected 0 memories, got %d", len(memories))
	}
}

func TestJSONStore_Delete(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	js.Write(&domain.Memory{
		ID: "del-1", Summary: "Delete me", Type: "context",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}, dir)

	ok, err := js.Delete("del-1", dir)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !ok {
		t.Error("expected Delete to return true")
	}

	// Should be gone
	got, _ := js.Read("del-1", dir)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestJSONStore_Delete_Nonexistent(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	ok, err := js.Delete("no-such", dir)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok {
		t.Error("expected false for nonexistent file")
	}
}

func TestJSONStore_ComputeFingerprint(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	// Empty dir
	fp1, _ := js.ComputeFingerprint(dir)

	// Add a file and check fingerprint changes
	js.Write(&domain.Memory{
		ID: "fp-1", Summary: "FP test", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}, dir)
	fp2, _ := js.ComputeFingerprint(dir)

	if fp2 == "" {
		t.Error("expected non-empty fingerprint")
	}
	if fp1 == fp2 {
		t.Error("fingerprint should change after adding file")
	}
}

func TestJSONStore_Processed(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	// Initially empty
	processed, err := js.ReadProcessed(dir)
	if err != nil {
		t.Fatalf("ReadProcessed: %v", err)
	}
	if len(processed) != 0 {
		t.Errorf("expected 0, got %d", len(processed))
	}

	// Add some
	err = js.AddProcessed(map[string]bool{"abc": true, "def": true}, dir)
	if err != nil {
		t.Fatalf("AddProcessed: %v", err)
	}

	processed, _ = js.ReadProcessed(dir)
	if len(processed) != 2 {
		t.Errorf("expected 2, got %d", len(processed))
	}
	if !processed["abc"] || !processed["def"] {
		t.Error("expected both hashes to be present")
	}

	// Add more — should merge
	js.AddProcessed(map[string]bool{"ghi": true}, dir)
	processed, _ = js.ReadProcessed(dir)
	if len(processed) != 3 {
		t.Errorf("expected 3 after merge, got %d", len(processed))
	}
}

func TestRebuildDBFromJSON(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	// Write memories
	js.Write(&domain.Memory{
		ID: "rb-1", Summary: "Rebuild 1", Type: "decision",
		Importance: 80, SourceCommits: []string{}, FilePaths: []string{"a.go"},
		Tags: []string{}, CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}, dir)
	js.Write(&domain.Memory{
		ID: "rb-2", Summary: "Rebuild 2", Type: "pattern",
		Importance: 60, SourceCommits: []string{}, FilePaths: []string{"b.go"},
		Tags: []string{}, CreatedAt: "2025-01-01T00:00:00Z", Active: true,
		Links: []map[string]any{
			{"target": "rb-1", "relationship": "supports", "strength": 0.8},
		},
	}, dir)

	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	link := storage.NewLinkStore(db)
	mem := storage.NewMemoryStore(db, link)

	count, err := storage.RebuildDBFromJSON(db, mem, link, js, dir, nil)
	if err != nil {
		t.Fatalf("RebuildDBFromJSON: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 memories rebuilt, got %d", count)
	}

	// Verify memories in DB
	m1, _ := mem.Get("rb-1")
	if m1 == nil || m1.Summary != "Rebuild 1" {
		t.Error("rb-1 not found or wrong summary")
	}

	// Verify links were restored
	links, _ := link.GetLinksFor("rb-2")
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
}

func TestRebuildDBFromJSON_WithFilter(t *testing.T) {
	dir := t.TempDir()
	js := storage.NewJSONStore()

	js.Write(&domain.Memory{
		ID: "filt-1", Summary: "Keep", Type: "decision",
		Importance: 80, SourceCommits: []string{}, FilePaths: []string{"src/main.go"},
		Tags: []string{}, CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}, dir)
	js.Write(&domain.Memory{
		ID: "filt-2", Summary: "Skip", Type: "debt",
		Importance: 20, SourceCommits: []string{}, FilePaths: []string{"vendor/lib.go"},
		Tags: []string{}, CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}, dir)

	db, _ := storage.NewDatabase(":memory:")
	defer db.Close()
	link := storage.NewLinkStore(db)
	mem := storage.NewMemoryStore(db, link)

	filter := func(m *domain.Memory) bool {
		for _, f := range m.FilePaths {
			if f == "vendor/lib.go" {
				return false
			}
		}
		return true
	}

	count, err := storage.RebuildDBFromJSON(db, mem, link, js, dir, filter)
	if err != nil {
		t.Fatalf("RebuildDBFromJSON: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 memory after filter, got %d", count)
	}
}
