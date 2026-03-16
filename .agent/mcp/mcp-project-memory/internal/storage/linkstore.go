package storage

import (
	"database/sql"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// LinkStore implements domain.LinkStore (LinkReader + LinkWriter).
type LinkStore struct {
	db *Database
}

// NewLinkStore creates a LinkStore backed by the given Database.
func NewLinkStore(db *Database) *LinkStore {
	return &LinkStore{db: db}
}

func rowToLink(rows *sql.Rows) (*domain.MemoryLink, error) {
	var l domain.MemoryLink
	if err := rows.Scan(&l.ID, &l.MemoryIDA, &l.MemoryIDB, &l.Relationship, &l.Strength, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

// CreateLink inserts a new link.
func (s *LinkStore) CreateLink(link *domain.MemoryLink) error {
	if link.CreatedAt == "" {
		link.CreatedAt = domain.NowUTC()
	}
	result, err := s.db.DB().Exec(
		`INSERT INTO memory_links (memory_id_a, memory_id_b, relationship, strength, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		link.MemoryIDA, link.MemoryIDB, link.Relationship, link.Strength, link.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	link.ID = int(id)
	return nil
}

// GetLinksFor returns all links involving a memory (bidirectional).
func (s *LinkStore) GetLinksFor(memoryID string) ([]*domain.MemoryLink, error) {
	rows, err := s.db.DB().Query(
		"SELECT id, memory_id_a, memory_id_b, relationship, strength, created_at FROM memory_links WHERE memory_id_a = ? OR memory_id_b = ?",
		memoryID, memoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []*domain.MemoryLink
	for rows.Next() {
		l, err := rowToLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetLinkedIDs returns UUIDs of all memories linked to the given memory.
func (s *LinkStore) GetLinkedIDs(memoryID string) ([]string, error) {
	links, err := s.GetLinksFor(memoryID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, l := range links {
		if l.MemoryIDA == memoryID {
			ids = append(ids, l.MemoryIDB)
		} else {
			ids = append(ids, l.MemoryIDA)
		}
	}
	return ids, nil
}

// DeleteForMemory deletes all links involving a memory.
func (s *LinkStore) DeleteForMemory(memoryID string) error {
	_, err := s.db.DB().Exec(
		"DELETE FROM memory_links WHERE memory_id_a = ? OR memory_id_b = ?",
		memoryID, memoryID,
	)
	return err
}

// ListAll lists all links up to the given limit.
func (s *LinkStore) ListAll(limit int) ([]*domain.MemoryLink, error) {
	rows, err := s.db.DB().Query(
		"SELECT id, memory_id_a, memory_id_b, relationship, strength, created_at FROM memory_links LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []*domain.MemoryLink
	for rows.Next() {
		l, err := rowToLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
