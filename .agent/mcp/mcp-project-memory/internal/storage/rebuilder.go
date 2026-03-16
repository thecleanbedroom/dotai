package storage

import "github.com/dotai/mcp-project-memory/internal/domain"

// DBRebuilder implements domain.Rebuilder, wrapping the RebuildDBFromJSON
// function with concrete storage types pre-wired.
type DBRebuilder struct {
	db   *Database
	mem  *MemoryStore
	link *LinkStore
	json domain.JSONStore
}

// NewRebuilder creates a Rebuilder backed by the given stores.
func NewRebuilder(db *Database, mem *MemoryStore, link *LinkStore, json domain.JSONStore) *DBRebuilder {
	return &DBRebuilder{db: db, mem: mem, link: link, json: json}
}

// RebuildFromJSON drops and re-creates the DB from JSON memory files.
func (r *DBRebuilder) RebuildFromJSON(filterFn func(*domain.Memory) bool) (int, error) {
	return RebuildDBFromJSON(r.db, r.mem, r.link, r.json, filterFn)
}
