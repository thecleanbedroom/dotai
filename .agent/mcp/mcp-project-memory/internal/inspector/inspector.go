// Package inspector provides debug/inspect commands for AI visibility
// into raw memory data. Depends on domain interfaces only.
package inspector

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// Inspector routes debug queries to the appropriate handler.
type Inspector struct {
	memReader domain.MemoryReader
	links     domain.LinkReader
	builds    domain.BuildMetaStore
	dbMgr     domain.DatabaseManager
	rawDB     RawQuerier
}

// RawQuerier provides raw SQL access for schema/table inspection.
type RawQuerier interface {
	Query(query string, args ...any) ([]map[string]any, error)
}

// New creates an Inspector.
func New(
	memReader domain.MemoryReader,
	links domain.LinkReader,
	builds domain.BuildMetaStore,
	dbMgr domain.DatabaseManager,
	rawDB RawQuerier,
) *Inspector {
	return &Inspector{
		memReader: memReader,
		links:     links,
		builds:    builds,
		dbMgr:     dbMgr,
		rawDB:     rawDB,
	}
}

// Inspect routes a query string to the appropriate handler.
func (ins *Inspector) Inspect(query string) string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(parts) == 0 {
		return ins.help()
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "tables":
		return ins.tables()
	case "memories":
		return ins.allMemories()
	case "memory":
		return ins.singleMemory(args)
	case "links":
		return ins.allLinks()
	case "builds":
		return ins.buildHistory()
	case "stats":
		return ins.stats()
	case "schema":
		return ins.schema()
	case "fts":
		return ins.ftsHealth()
	case "help":
		return ins.help()
	default:
		return fmt.Sprintf("Unknown inspect command: %s\n\n%s", cmd, ins.help())
	}
}

func (ins *Inspector) help() string {
	return `Inspect commands:
  tables            — List all tables
  memories          — Show all memories
  memory <id>       — Show a specific memory with links
  links             — Show all links
  builds            — Show build history
  stats             — Aggregate statistics
  schema            — Show table schemas
  fts               — FTS5 index health check
  help              — This message`
}

func (ins *Inspector) tables() string {
	if ins.rawDB == nil {
		return `{"error": "raw query not available"}`
	}
	rows, err := ins.rawDB.Query("SELECT name, type FROM sqlite_master WHERE type IN ('table', 'view') ORDER BY name")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	return string(data)
}

func (ins *Inspector) allMemories() string {
	memories, err := ins.memReader.ListAll(false, 200)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	var dicts []map[string]any
	for _, m := range memories {
		dicts = append(dicts, m.ToDict())
	}
	data, _ := json.MarshalIndent(dicts, "", "  ")
	return string(data)
}

func (ins *Inspector) singleMemory(args []string) string {
	if len(args) == 0 {
		return "Usage: memory <id>"
	}
	m, err := ins.memReader.Get(args[0])
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	if m == nil {
		return fmt.Sprintf("Memory %s not found", args[0])
	}
	links, _ := ins.links.GetLinksFor(args[0])
	result := map[string]any{
		"memory": m.ToDict(),
		"links":  links,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data)
}

func (ins *Inspector) allLinks() string {
	links, err := ins.links.ListAll(200)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	data, _ := json.MarshalIndent(links, "", "  ")
	return string(data)
}

func (ins *Inspector) buildHistory() string {
	builds, err := ins.builds.ListBuilds(20)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	data, _ := json.MarshalIndent(builds, "", "  ")
	return string(data)
}

func (ins *Inspector) stats() string {
	stats, err := ins.memReader.Stats()
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	lastBuild, _ := ins.builds.GetLast()
	stats["last_build"] = lastBuild
	data, _ := json.MarshalIndent(stats, "", "  ")
	return string(data)
}

func (ins *Inspector) schema() string {
	if ins.rawDB == nil {
		return `{"error": "raw query not available"}`
	}
	rows, err := ins.rawDB.Query("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	var stmts []string
	for _, r := range rows {
		if s, ok := r["sql"].(string); ok {
			stmts = append(stmts, s)
		}
	}
	return strings.Join(stmts, "\n\n")
}

func (ins *Inspector) ftsHealth() string {
	if ins.rawDB == nil {
		return `{"error": "raw query not available"}`
	}
	ftsRows, err := ins.rawDB.Query("SELECT COUNT(*) as c FROM memories_fts")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	var ftsCount int
	if len(ftsRows) > 0 {
		if c, ok := ftsRows[0]["c"].(int64); ok {
			ftsCount = int(c)
		}
	}
	memCount, _ := ins.memReader.Count(false)
	result := map[string]any{
		"fts_rows":    ftsCount,
		"memory_rows": memCount,
		"in_sync":     ftsCount == memCount,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data)
}
