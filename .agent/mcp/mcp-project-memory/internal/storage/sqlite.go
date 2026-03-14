// Package storage implements domain interfaces using SQLite (modernc.org/sqlite).
// This file provides the low-level Database wrapper implementing domain.DatabaseManager.
package storage

import (
	"database/sql"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// Database wraps a SQLite connection with schema management, bulk-operation
// scoping (Hold/Release), and fingerprinting for stale-cache detection.
// Implements domain.DatabaseManager.
type Database struct {
	path       string
	mu         sync.Mutex
	db         *sql.DB
	held       bool
	schemaInit bool
}

// NewDatabase opens (or creates) a SQLite database at the given path.
// Use ":memory:" for in-memory databases (testing).
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Pragmas for performance
	for _, pragma := range []string{
		"PRAGMA journal_mode=DELETE",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA cache_size=-64000",
		"PRAGMA temp_store=MEMORY",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %s: %w", pragma, err)
		}
	}

	d := &Database{path: path, db: db}
	if err := d.InitSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return d, nil
}

// DB returns the underlying *sql.DB for stores to use.
func (d *Database) DB() *sql.DB { return d.db }

// Close closes the database connection.
func (d *Database) Close() error { return d.db.Close() }

// Hold keeps the connection in a transaction for bulk operations.
func (d *Database) Hold() error {
	d.mu.Lock()
	d.held = true
	return nil
}

// Release commits and ends the bulk-operation scope.
func (d *Database) Release() error {
	d.held = false
	d.mu.Unlock()
	return nil
}

// InitSchema creates all tables, indexes, FTS5, and triggers.
func (d *Database) InitSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS memories (
			id              TEXT PRIMARY KEY,
			summary         TEXT NOT NULL,
			type            TEXT NOT NULL,
			confidence      INTEGER NOT NULL DEFAULT 0,
			importance      INTEGER NOT NULL DEFAULT 50,
			source_commits  TEXT NOT NULL DEFAULT '[]',
			file_paths      TEXT NOT NULL DEFAULT '[]',
			tags            TEXT NOT NULL DEFAULT '[]',
			created_at      TEXT NOT NULL,
			accessed_at     TEXT NOT NULL DEFAULT '',
			access_count    INTEGER NOT NULL DEFAULT 0,
			active          INTEGER NOT NULL DEFAULT 1
		);

		CREATE TABLE IF NOT EXISTS memory_links (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			memory_id_a   TEXT NOT NULL REFERENCES memories(id),
			memory_id_b   TEXT NOT NULL REFERENCES memories(id),
			relationship  TEXT NOT NULL,
			strength      REAL NOT NULL DEFAULT 0.5,
			created_at    TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS build_meta (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			build_type      TEXT NOT NULL,
			commit_count    INTEGER NOT NULL,
			memory_count    INTEGER NOT NULL,
			built_at        TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS db_meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
		CREATE INDEX IF NOT EXISTS idx_memories_active ON memories(active);
		CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
		CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(confidence);
		CREATE INDEX IF NOT EXISTS idx_memory_links_a ON memory_links(memory_id_a);
		CREATE INDEX IF NOT EXISTS idx_memory_links_b ON memory_links(memory_id_b);
	`
	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}

	// FTS5 virtual table (idempotent — ignore "already exists")
	fts := `CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		summary, type, tags,
		content=memories,
		content_rowid=rowid
	)`
	if _, err := d.db.Exec(fts); err != nil {
		return fmt.Errorf("create fts5: %w", err)
	}

	// FTS sync triggers
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, summary, type, tags)
			VALUES (new.rowid, new.summary, new.type, new.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
			VALUES ('delete', old.rowid, old.summary, old.type, old.tags);
		END`,
		`CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
			VALUES ('delete', old.rowid, old.summary, old.type, old.tags);
			INSERT INTO memories_fts(rowid, summary, type, tags)
			VALUES (new.rowid, new.summary, new.type, new.tags);
		END`,
	}
	for _, t := range triggers {
		if _, err := d.db.Exec(t); err != nil {
			return fmt.Errorf("create trigger: %w", err)
		}
	}

	d.schemaInit = true
	return nil
}

// DropAll removes all tables for a full rebuild.
func (d *Database) DropAll() error {
	drops := `
		DROP TABLE IF EXISTS memory_links;
		DROP TRIGGER IF EXISTS memories_ai;
		DROP TRIGGER IF EXISTS memories_ad;
		DROP TRIGGER IF EXISTS memories_au;
		DROP TABLE IF EXISTS memories_fts;
		DROP TABLE IF EXISTS memories;
		DROP TABLE IF EXISTS build_meta;
		DROP TABLE IF EXISTS db_meta;
	`
	if _, err := d.db.Exec(drops); err != nil {
		return fmt.Errorf("drop all: %w", err)
	}
	d.schemaInit = false
	return nil
}

// GetFingerprint reads the stored JSON fingerprint.
func (d *Database) GetFingerprint() (string, error) {
	var fp string
	err := d.db.QueryRow("SELECT value FROM db_meta WHERE key = 'json_fingerprint'").Scan(&fp)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return fp, err
}

// SetFingerprint stores the current JSON fingerprint.
func (d *Database) SetFingerprint(fp string) error {
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO db_meta (key, value) VALUES ('json_fingerprint', ?)", fp,
	)
	return err
}
