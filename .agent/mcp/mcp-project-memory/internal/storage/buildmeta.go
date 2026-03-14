package storage

import (
	"database/sql"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// BuildMetaStore implements domain.BuildMetaStore.
type BuildMetaStore struct {
	db *Database
}

// NewBuildMetaStore creates a BuildMetaStore backed by the given Database.
func NewBuildMetaStore(db *Database) *BuildMetaStore {
	return &BuildMetaStore{db: db}
}

// Record inserts a new build run entry.
func (s *BuildMetaStore) Record(entry *domain.BuildMetaEntry) error {
	if entry.BuiltAt == "" {
		entry.BuiltAt = domain.NowUTC()
	}
	result, err := s.db.DB().Exec(
		`INSERT INTO build_meta (build_type, commit_count, memory_count, built_at)
		 VALUES (?, ?, ?, ?)`,
		entry.BuildType, entry.CommitCount, entry.MemoryCount, entry.BuiltAt,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	entry.ID = int(id)
	return nil
}

// GetLast returns the most recent build metadata entry.
func (s *BuildMetaStore) GetLast() (*domain.BuildMetaEntry, error) {
	var e domain.BuildMetaEntry
	err := s.db.DB().QueryRow(
		"SELECT id, build_type, commit_count, memory_count, built_at FROM build_meta ORDER BY id DESC LIMIT 1",
	).Scan(&e.ID, &e.BuildType, &e.CommitCount, &e.MemoryCount, &e.BuiltAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListBuilds returns recent builds in descending order.
func (s *BuildMetaStore) ListBuilds(limit int) ([]*domain.BuildMetaEntry, error) {
	rows, err := s.db.DB().Query(
		"SELECT id, build_type, commit_count, memory_count, built_at FROM build_meta ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*domain.BuildMetaEntry
	for rows.Next() {
		var e domain.BuildMetaEntry
		if err := rows.Scan(&e.ID, &e.BuildType, &e.CommitCount, &e.MemoryCount, &e.BuiltAt); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}
