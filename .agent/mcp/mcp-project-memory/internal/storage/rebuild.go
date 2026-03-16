package storage

import (
	"fmt"
	"os"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// RebuildDBFromJSON drops all tables and reloads from JSON memory files.
// Returns the number of memories loaded.
func RebuildDBFromJSON(db *Database, memStore *MemoryStore, linkStore *LinkStore, jsonStore domain.JSONStore, filterFn func(*domain.Memory) bool) (int, error) {
	if err := db.DropAll(); err != nil {
		return 0, fmt.Errorf("drop all: %w", err)
	}
	if err := db.InitSchema(); err != nil {
		return 0, fmt.Errorf("init schema: %w", err)
	}

	memories, err := jsonStore.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read all: %w", err)
	}

	// Apply path filtering if provided
	if filterFn != nil {
		var filtered []*domain.Memory
		for _, m := range memories {
			if filterFn(m) {
				filtered = append(filtered, m)
			}
		}
		dropped := len(memories) - len(filtered)
		if dropped > 0 {
			fmt.Fprintf(os.Stderr, "  path filter: dropped %d memories (all paths ignored)\n", dropped)
		}
		memories = filtered
	}

	// Insert memories
	for _, m := range memories {
		if err := memStore.Create(m); err != nil {
			return 0, fmt.Errorf("create memory %s: %w", m.ID, err)
		}
	}

	// Restore links from memory JSON files
	validIDs := make(map[string]bool, len(memories))
	for _, m := range memories {
		validIDs[m.ID] = true
	}
	for _, m := range memories {
		for _, linkData := range m.Links {
			target := stringFromMap(linkData, "target")
			if target == "" {
				target = stringFromMap(linkData, "memory_id_b")
			}
			if target != "" && validIDs[target] {
				link := &domain.MemoryLink{
					MemoryIDA:    m.ID,
					MemoryIDB:    target,
					Relationship: stringFromMapDefault(linkData, "relationship", "related_to"),
					Strength:     floatFromMap(linkData, "strength", 0.5),
				}
				if err := linkStore.CreateLink(link); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: create link %s→%s: %v\n", link.MemoryIDA, link.MemoryIDB, err)
				}
			}
		}
	}

	// Update fingerprint
	fp, err := jsonStore.ComputeFingerprint()
	if err == nil && fp != "" {
		if err := db.SetFingerprint(fp); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: set fingerprint: %v\n", err)
		}
	}

	// Seed processed_commits from memory source_commits
	commitSet := map[string]bool{}
	for _, m := range memories {
		for _, h := range m.SourceCommits {
			commitSet[h] = true
		}
	}
	if len(commitSet) > 0 {
		if err := db.AddProcessed(commitSet); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: seed processed commits: %v\n", err)
		}
	}

	return len(memories), nil
}

func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func stringFromMapDefault(m map[string]any, key, def string) string {
	if v := stringFromMap(m, key); v != "" {
		return v
	}
	return def
}

func floatFromMap(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}
