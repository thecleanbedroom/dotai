package domain

// --- Segregated interfaces (Interface Segregation Principle) ---
// Consumers import only the interfaces they need. Concrete implementations
// in storage/, git/, llm/ satisfy these contracts.

// MemoryReader provides read-only access to memories.
type MemoryReader interface {
	Get(id string) (*Memory, error)
	GetMany(ids []string) ([]*Memory, error)
	QueryByFile(path string, limit, minImportance int) ([]*Memory, error)
	ListAll(activeOnly bool, limit int) ([]*Memory, error)
	Count(activeOnly bool) (int, error)
	Stats() (map[string]any, error)
}

// MemoryWriter provides write access to memories.
type MemoryWriter interface {
	Create(m *Memory) error
	Update(m *Memory) error
	Deactivate(id string) error
	Touch(id string) error
}

// Searcher provides full-text search over memories.
type Searcher interface {
	Search(query string, opts SearchOpts) ([]*Memory, error)
}

// LinkReader provides read-only access to memory links.
type LinkReader interface {
	GetLinksFor(memoryID string) ([]*MemoryLink, error)
	GetLinkedIDs(memoryID string) ([]string, error)
	ListAll(limit int) ([]*MemoryLink, error)
}

// LinkWriter provides write access to memory links.
type LinkWriter interface {
	CreateLink(link *MemoryLink) error
	DeleteForMemory(memoryID string) error
}

// LinkStore combines read and write access to links.
type LinkStore interface {
	LinkReader
	LinkWriter
}

// BuildMetaStore manages build run metadata.
type BuildMetaStore interface {
	Record(entry *BuildMetaEntry) error
	GetLast() (*BuildMetaEntry, error)
	ListBuilds(limit int) ([]*BuildMetaEntry, error)
}

// ProcessedTracker tracks which git commits have been processed.
type ProcessedTracker interface {
	ReadProcessed() (map[string]bool, error)
	AddProcessed(hashes map[string]bool) error
	ClearProcessed() error
}

// JSONStore handles JSON file I/O for the canonical memory store.
type JSONStore interface {
	ReadAll() ([]*Memory, error)
	Read(id string) (*Memory, error)
	Write(m *Memory) error
	Delete(id string) (bool, error)
	ComputeFingerprint() (string, error)
}

// GitParser extracts commit data from a git repository.
type GitParser interface {
	GetAllHashes(limit int) ([]string, error)
	GetCommitsByHashes(hashes []string) ([]*ParsedCommit, error)
	GetCurrentHash() (string, error)
}

// LLMCaller handles LLM chat completion calls.
type LLMCaller interface {
	Chat(messages []Message, opts ChatOpts) (string, error)
	GetModelInfo() (ModelInfo, error)
}

// DatabaseManager provides low-level DB operations (schema, fingerprint).
type DatabaseManager interface {
	DropAll() error
	InitSchema() error
	GetFingerprint() (string, error)
	SetFingerprint(fp string) error
}

// Rebuilder rebuilds the SQLite DB from canonical JSON memory files.
// Used by the MCP server to refresh stale data without importing storage.
type Rebuilder interface {
	RebuildFromJSON(filterFn func(*Memory) bool) (int, error)
}
