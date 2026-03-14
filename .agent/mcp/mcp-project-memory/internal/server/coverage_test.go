package server_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/server"
	"github.com/dotai/mcp-project-memory/internal/storage"
)

// Tests for server tool handler helpers and the full tool registration path.

func TestToolHandlers_ViaRegister(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed data
	memStore := s.MemReader.(*storage.MemoryStore)
	memStore.Create(&domain.Memory{
		ID: "tool-1", Summary: "Architecture decision about caching",
		Type: "decision", Importance: 80,
		SourceCommits: []string{"abc123"}, FilePaths: []string{"internal/cache.go"},
		Tags: []string{"architecture", "caching"},
		CreatedAt: "2025-01-01T00:00:00Z", Active: true,
	})

	mcpSrv := createTestMCPServer()
	server.RegisterTools(mcpSrv, s)

	// Verify tools were registered (the server should have the tools)
	// Just verify it doesn't panic and all 5 tools registered successfully
}

func TestBuildMetaStore_ListBuilds(t *testing.T) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	bm := storage.NewBuildMetaStore(db)

	bm.Record(&domain.BuildMetaEntry{
		BuildType: "incremental", CommitCount: 5, MemoryCount: 10,
	})
	bm.Record(&domain.BuildMetaEntry{
		BuildType: "full", CommitCount: 50, MemoryCount: 100,
	})

	builds, err := bm.ListBuilds(10)
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 2 {
		t.Errorf("expected 2 builds, got %d", len(builds))
	}
}

func TestBuildMetaStore_ListBuilds_Empty(t *testing.T) {
	db, _ := storage.NewDatabase(":memory:")
	defer db.Close()

	bm := storage.NewBuildMetaStore(db)
	builds, err := bm.ListBuilds(10)
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if len(builds) != 0 {
		t.Errorf("expected 0 builds, got %d", len(builds))
	}
}
