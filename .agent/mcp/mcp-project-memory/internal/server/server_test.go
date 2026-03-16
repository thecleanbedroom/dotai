package server_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/inspector"
	"github.com/dotai/mcp-project-memory/internal/server"
	"github.com/dotai/mcp-project-memory/internal/storage"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func createTestMCPServer() *mcpserver.MCPServer {
	return mcpserver.NewMCPServer("test-memory", "0.0.1-test")
}

func setupTestServer(t *testing.T) (*server.McpServer, *storage.Database) {
	t.Helper()
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	linkStore := storage.NewLinkStore(db)
	memStore := storage.NewMemoryStore(db, linkStore)
	buildMeta := storage.NewBuildMetaStore(db)

	tempDir := t.TempDir()
	paths := storage.NewPaths(tempDir)
	jsonStore := storage.NewJSONStore(paths)
	rebuilder := storage.NewRebuilder(db, memStore, linkStore, jsonStore)

	insp := inspector.New(memStore, linkStore, buildMeta, db, &testRawQuerier{db: db})

	s := &server.McpServer{
		MemReader: memStore,
		MemWriter: memStore,
		Searcher:  memStore,
		Links:     linkStore,
		Builds:    buildMeta,
		Inspector: insp,
		DB:        db,
		JSONStore:  jsonStore,
		Rebuilder: rebuilder,
	}
	return s, db
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

func TestEnsureFresh_EmptyData(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// With an empty data dir, EnsureFresh may or may not rebuild
	// depending on whether ComputeFingerprint returns "" for empty dirs.
	// The important thing is it doesn't error.
	_, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
}

func TestEnsureFresh_UpToDate(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Write a memory and set matching fingerprint
	js := s.JSONStore
	m := &domain.Memory{
		ID: "fresh-1", Summary: "Test", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}
	js.Write(m)

	fp, _ := js.ComputeFingerprint()
	s.DB.SetFingerprint(fp)

	rebuilt, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if rebuilt {
		t.Error("should not rebuild when fingerprints match")
	}
}

func TestEnsureFresh_Stale(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Write memory
	js := s.JSONStore
	m := &domain.Memory{
		ID: "stale-1", Summary: "Stale", Type: "decision",
		SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	}
	js.Write(m)

	// Set a different fingerprint
	s.DB.SetFingerprint("old-fingerprint")

	rebuilt, err := s.EnsureFresh()
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if !rebuilt {
		t.Error("should rebuild when fingerprints differ")
	}

	// After rebuild, memory should be in DB
	got, _ := s.MemReader.Get("stale-1")
	if got == nil {
		t.Error("memory should be in DB after rebuild")
	}
}

func TestRegisterTools_DoesNotPanic(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Just verify registration doesn't panic
	mcpSrv := createTestMCPServer()
	server.RegisterTools(mcpSrv, s)
}
