package storage_test

import (
	"testing"

	"github.com/dotai/mcp-project-memory/internal/storage"
)

func TestNewDatabase_InMemory(t *testing.T) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase(:memory:): %v", err)
	}
	defer db.Close()

	// Schema should be initialized
	var count int
	err = db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count < 4 {
		t.Errorf("expected at least 4 tables, got %d", count)
	}
}

func TestFingerprint(t *testing.T) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	// Initially empty
	fp, err := db.GetFingerprint()
	if err != nil {
		t.Fatalf("GetFingerprint: %v", err)
	}
	if fp != "" {
		t.Errorf("expected empty fingerprint, got %q", fp)
	}

	// Set and read back
	if err := db.SetFingerprint("abc123"); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	fp, err = db.GetFingerprint()
	if err != nil {
		t.Fatalf("GetFingerprint: %v", err)
	}
	if fp != "abc123" {
		t.Errorf("expected 'abc123', got %q", fp)
	}
}

func TestDropAllAndReinit(t *testing.T) {
	db, err := storage.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer db.Close()

	if err := db.DropAll(); err != nil {
		t.Fatalf("DropAll: %v", err)
	}

	// User tables should be gone (sqlite_sequence may remain)
	var count int
	db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&count)
	if count > 0 {
		t.Errorf("expected 0 user tables after drop, got %d", count)
	}

	// Re-init
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	db.DB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if count < 4 {
		t.Errorf("expected at least 4 tables after re-init, got %d", count)
	}
}
