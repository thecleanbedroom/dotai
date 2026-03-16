package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dotai/mcp-project-memory/internal/storage"
)

func runReset(paths *storage.Paths) {
	fmt.Fprintln(os.Stderr, "resetting project memory...")

	// Remove memory JSON files
	entries, err := os.ReadDir(paths.MemoriesDir)
	if err != nil && !os.IsNotExist(err) {
		slog.Warn("reset: read memories dir", "err", err)
	}
	removed := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			if err := os.Remove(filepath.Join(paths.MemoriesDir, e.Name())); err != nil {
				slog.Warn("reset: remove memory file", "file", e.Name(), "err", err)
			}
			removed++
		}
	}

	// Remove links.json
	if err := os.Remove(filepath.Join(paths.DataDir, "links.json")); err != nil && !os.IsNotExist(err) {
		slog.Warn("reset: remove links.json", "err", err)
	}

	// Recreate DB
	if err := os.Remove(paths.DBFile); err != nil && !os.IsNotExist(err) {
		slog.Warn("reset: remove DB file", "err", err)
	}
	db, err := storage.NewDatabase(paths.DBFile)
	if err != nil {
		log.Fatalf("recreate database: %v", err)
	}
	db.Close()

	fmt.Fprintf(os.Stderr, "reset complete: removed %d memory files, recreated DB\n", removed)
}
