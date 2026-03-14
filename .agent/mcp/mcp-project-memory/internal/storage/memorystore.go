package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// MemoryStore implements domain.MemoryReader, domain.MemoryWriter, and domain.Searcher.
type MemoryStore struct {
	db   *Database
	link *LinkStore // for enrichWithLinks
}

// NewMemoryStore creates a MemoryStore backed by the given Database.
func NewMemoryStore(db *Database, link *LinkStore) *MemoryStore {
	return &MemoryStore{db: db, link: link}
}

func rowToMemory(row interface {
	Scan(dest ...any) error
}, cols []string,
) (*domain.Memory, error) {
	var (
		id, summary, memType, createdAt, accessedAt string
		confidence, importance, accessCount          int
		active                                       int
		sourceCommitsJSON, filePathsJSON, tagsJSON   string
	)
	if err := row.Scan(
		&id, &summary, &memType, &confidence, &importance,
		&sourceCommitsJSON, &filePathsJSON, &tagsJSON,
		&createdAt, &accessedAt, &accessCount, &active,
	); err != nil {
		return nil, err
	}

	m := &domain.Memory{
		ID:          id,
		Summary:     summary,
		Type:        memType,
		Confidence:  confidence,
		Importance:  importance,
		CreatedAt:   createdAt,
		AccessedAt:  accessedAt,
		AccessCount: accessCount,
		Active:      active != 0,
	}

	_ = json.Unmarshal([]byte(sourceCommitsJSON), &m.SourceCommits)
	_ = json.Unmarshal([]byte(filePathsJSON), &m.FilePaths)
	_ = json.Unmarshal([]byte(tagsJSON), &m.Tags)

	if m.SourceCommits == nil {
		m.SourceCommits = []string{}
	}
	if m.FilePaths == nil {
		m.FilePaths = []string{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}

	return m, nil
}

const memoryCols = "id, summary, type, confidence, importance, source_commits, file_paths, tags, created_at, accessed_at, access_count, active"

func scanMemories(rows *sql.Rows) ([]*domain.Memory, error) {
	defer rows.Close()
	var result []*domain.Memory
	for rows.Next() {
		m, err := rowToMemory(rows, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// enrichWithLinks populates the Links field on memories from the memory_links table.
func (s *MemoryStore) enrichWithLinks(memories []*domain.Memory) []*domain.Memory {
	if len(memories) == 0 || s.link == nil {
		return memories
	}

	ids := make([]string, len(memories))
	for i, m := range memories {
		ids[i] = m.ID
	}
	ph := placeholders(len(ids))
	args := toAnySlice(ids)

	// Outgoing
	outRows, _ := s.db.DB().Query(
		fmt.Sprintf("SELECT memory_id_a, memory_id_b, relationship, strength FROM memory_links WHERE memory_id_a IN (%s)", ph),
		args...,
	)
	linksByID := map[string][]map[string]any{}
	if outRows != nil {
		defer outRows.Close()
		for outRows.Next() {
			var a, b, rel string
			var str float64
			outRows.Scan(&a, &b, &rel, &str)
			linksByID[a] = append(linksByID[a], map[string]any{
				"target": b, "relationship": rel, "strength": str,
			})
		}
	}

	// Incoming
	inRows, _ := s.db.DB().Query(
		fmt.Sprintf("SELECT memory_id_a, memory_id_b, relationship, strength FROM memory_links WHERE memory_id_b IN (%s)", ph),
		args...,
	)
	if inRows != nil {
		defer inRows.Close()
		for inRows.Next() {
			var a, b, rel string
			var str float64
			inRows.Scan(&a, &b, &rel, &str)
			linksByID[b] = append(linksByID[b], map[string]any{
				"target": a, "relationship": rel, "strength": str, "direction": "incoming",
			})
		}
	}

	for _, m := range memories {
		m.Links = linksByID[m.ID]
	}
	return memories
}

// Create inserts a new memory.
func (s *MemoryStore) Create(m *domain.Memory) error {
	if m.AccessedAt == "" {
		m.AccessedAt = domain.NowUTC()
	}
	sc, _ := json.Marshal(m.SourceCommits)
	fp, _ := json.Marshal(m.FilePaths)
	tg, _ := json.Marshal(m.Tags)

	_, err := s.db.DB().Exec(
		`INSERT INTO memories (id, summary, type, confidence, importance, source_commits,
		 file_paths, tags, created_at, accessed_at, access_count, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Summary, m.Type, m.Confidence, m.Importance,
		string(sc), string(fp), string(tg),
		m.CreatedAt, m.AccessedAt, m.AccessCount, boolToInt(m.Active),
	)
	return err
}

// Get retrieves a memory by ID with links enriched.
func (s *MemoryStore) Get(id string) (*domain.Memory, error) {
	row := s.db.DB().QueryRow(
		fmt.Sprintf("SELECT %s FROM memories WHERE id = ?", memoryCols), id,
	)
	m, err := rowToMemory(row, nil)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.enrichWithLinks([]*domain.Memory{m})
	return m, nil
}

// GetMany retrieves multiple memories by ID.
func (s *MemoryStore) GetMany(ids []string) ([]*domain.Memory, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ph := placeholders(len(ids))
	rows, err := s.db.DB().Query(
		fmt.Sprintf("SELECT %s FROM memories WHERE id IN (%s)", memoryCols, ph),
		toAnySlice(ids)...,
	)
	if err != nil {
		return nil, err
	}
	return scanMemories(rows)
}

// Update overwrites an existing memory's fields.
func (s *MemoryStore) Update(m *domain.Memory) error {
	sc, _ := json.Marshal(m.SourceCommits)
	fp, _ := json.Marshal(m.FilePaths)
	tg, _ := json.Marshal(m.Tags)

	_, err := s.db.DB().Exec(
		`UPDATE memories SET summary=?, type=?, confidence=?, importance=?,
		 source_commits=?, file_paths=?, tags=?, accessed_at=?,
		 access_count=?, active=? WHERE id=?`,
		m.Summary, m.Type, m.Confidence, m.Importance,
		string(sc), string(fp), string(tg),
		m.AccessedAt, m.AccessCount, boolToInt(m.Active), m.ID,
	)
	return err
}

// Deactivate soft-deletes a memory.
func (s *MemoryStore) Deactivate(id string) error {
	_, err := s.db.DB().Exec("UPDATE memories SET active = 0 WHERE id = ?", id)
	return err
}

// Touch records an access.
func (s *MemoryStore) Touch(id string) error {
	_, err := s.db.DB().Exec(
		"UPDATE memories SET accessed_at = ?, access_count = access_count + 1 WHERE id = ?",
		domain.NowUTC(), id,
	)
	return err
}

// QueryByFile retrieves memories associated with a file path.
func (s *MemoryStore) QueryByFile(path string, limit, minImportance int) ([]*domain.Memory, error) {
	pattern := fmt.Sprintf(`%%"%s%%`, path)
	rows, err := s.db.DB().Query(
		fmt.Sprintf(`SELECT %s FROM memories WHERE active = 1 AND file_paths LIKE ? AND importance >= ? ORDER BY importance DESC LIMIT ?`, memoryCols),
		pattern, minImportance, limit,
	)
	if err != nil {
		return nil, err
	}
	mems, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	return s.enrichWithLinks(mems), nil
}

// Search performs FTS5 search with filters.
func (s *MemoryStore) Search(query string, opts domain.SearchOpts) ([]*domain.Memory, error) {
	ftsQuery := prepareFTSQuery(query, opts.Match)

	clauses := []string{"memories_fts MATCH ?", "m.active = 1", "m.importance >= ?"}
	params := []any{ftsQuery, opts.MinImportance}

	if opts.Type != "" {
		clauses = append(clauses, "m.type = ?")
		params = append(params, opts.Type)
	}
	if opts.Since != "" {
		clauses = append(clauses, "m.created_at >= ?")
		params = append(params, opts.Since)
	}
	if opts.Until != "" {
		clauses = append(clauses, "m.created_at <= ?")
		params = append(params, opts.Until)
	}
	for _, tag := range opts.ExcludeTags {
		clauses = append(clauses, "m.tags NOT LIKE ?")
		params = append(params, fmt.Sprintf(`%%"%s"%%`, tag))
	}

	where := strings.Join(clauses, " AND ")
	params = append(params, opts.Limit)

	rows, err := s.db.DB().Query(
		fmt.Sprintf(`SELECT m.%s FROM memories m JOIN memories_fts f ON m.rowid = f.rowid WHERE %s ORDER BY rank LIMIT ?`,
			strings.ReplaceAll(memoryCols, ", ", ", m."), where),
		params...,
	)
	if err != nil {
		return nil, err
	}
	mems, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	return s.enrichWithLinks(mems), nil
}

// ListAll lists memories ordered by importance.
func (s *MemoryStore) ListAll(activeOnly bool, limit int) ([]*domain.Memory, error) {
	q := fmt.Sprintf("SELECT %s FROM memories", memoryCols)
	if activeOnly {
		q += " WHERE active = 1"
	}
	q += " ORDER BY importance DESC LIMIT ?"
	rows, err := s.db.DB().Query(q, limit)
	if err != nil {
		return nil, err
	}
	return scanMemories(rows)
}

// Count returns the number of memories.
func (s *MemoryStore) Count(activeOnly bool) (int, error) {
	q := "SELECT COUNT(*) FROM memories"
	if activeOnly {
		q += " WHERE active = 1"
	}
	var c int
	err := s.db.DB().QueryRow(q).Scan(&c)
	return c, err
}

// Stats returns aggregate statistics.
func (s *MemoryStore) Stats() (map[string]any, error) {
	var total int
	s.db.DB().QueryRow("SELECT COUNT(*) FROM memories WHERE active = 1").Scan(&total)

	byType := map[string]int{}
	rows, _ := s.db.DB().Query("SELECT type, COUNT(*) FROM memories WHERE active = 1 GROUP BY type")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c)
			byType[t] = c
		}
	}

	var avgConf, minConf, maxConf sql.NullFloat64
	s.db.DB().QueryRow("SELECT AVG(confidence), MIN(confidence), MAX(confidence) FROM memories WHERE active = 1").Scan(&avgConf, &minConf, &maxConf)

	var avgImp sql.NullFloat64
	s.db.DB().QueryRow("SELECT AVG(importance) FROM memories WHERE active = 1").Scan(&avgImp)

	// Top files
	topFiles := map[string]int{}
	fileRows, _ := s.db.DB().Query("SELECT file_paths FROM memories WHERE active = 1")
	if fileRows != nil {
		defer fileRows.Close()
		for fileRows.Next() {
			var fp string
			fileRows.Scan(&fp)
			var paths []string
			json.Unmarshal([]byte(fp), &paths)
			for _, p := range paths {
				topFiles[p]++
			}
		}
	}
	// Sort and limit to top 10
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range topFiles {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].v > sorted[i].v {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if len(sorted) > 10 {
		sorted = sorted[:10]
	}
	topFilesMap := map[string]int{}
	for _, s := range sorted {
		topFilesMap[s.k] = s.v
	}

	return map[string]any{
		"total_memories": total,
		"by_type":        byType,
		"confidence": map[string]any{
			"avg": nullFloat(avgConf),
			"min": nullFloatInt(minConf),
			"max": nullFloatInt(maxConf),
		},
		"avg_importance": nullFloatInt(avgImp),
		"top_files":      topFilesMap,
	}, nil
}

// --- Helpers ---

var nonWordRe = regexp.MustCompile(`[^\w\s]`)

func prepareFTSQuery(query, match string) string {
	cleaned := nonWordRe.ReplaceAllString(query, " ")
	terms := strings.Fields(cleaned)
	if len(terms) == 0 {
		return `""`
	}
	joiner := " AND "
	if match == "any" {
		joiner = " OR "
	}
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = t + "*"
	}
	return strings.Join(parts, joiner)
}

func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = "?"
	}
	return strings.Join(p, ",")
}

func toAnySlice(ss []string) []any {
	a := make([]any, len(ss))
	for i, s := range ss {
		a[i] = s
	}
	return a
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullFloat(n sql.NullFloat64) float64 {
	if n.Valid {
		return float64(int(n.Float64*10)) / 10
	}
	return 0
}

func nullFloatInt(n sql.NullFloat64) int {
	if n.Valid {
		return int(n.Float64)
	}
	return 0
}
