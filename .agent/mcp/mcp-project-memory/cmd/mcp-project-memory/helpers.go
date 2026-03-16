package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/storage"
)

// parseArgs extracts the command and --synthesis flag from os.Args.
// Default command is "serve" (no args required).
func parseArgs() (cmd string, synthesis bool) {
	cmd = "serve" // default to serve
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--synthesis":
			synthesis = true
		case "serve", "build", "reset", "models":
			cmd = arg
		default:
			if len(arg) > 0 && arg[0] != '-' {
				cmd = arg
			}
		}
	}
	return
}

func detectGitRoot(startDir string) string {
	dir := startDir
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: use startDir
	return startDir
}

// persistEnvKey writes or updates a key=value in the .env file.
func persistEnvKey(envPath, key, value string) {
	line := key + "=" + value + "\n"

	// Try to read existing file
	data, err := os.ReadFile(envPath)
	if err != nil {
		// File doesn't exist — create with just this key
		if err := os.WriteFile(envPath, []byte(line), 0o600); err != nil {
			slog.Warn("persist env key: create file", "err", err)
		}
		return
	}

	// Check if key already exists
	content := string(data)
	lines := strings.Split(content, "\n")
	found := false
	for i, l := range lines {
		if strings.HasPrefix(l, key+"=") || strings.HasPrefix(l, "# "+key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}

	if found {
		if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
			slog.Warn("persist env key: write file", "err", err)
		}
	} else {
		// Append
		f, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			slog.Warn("persist env key: open file", "path", envPath, "err", err)
			return
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + line); err != nil {
			slog.Warn("persist env key: append", "err", err)
		}
	}
}

// sqliteRawQuerier adapts *storage.Database to inspector.RawQuerier.
type sqliteRawQuerier struct {
	db *storage.Database
}

func (q *sqliteRawQuerier) Query(query string, args ...any) ([]map[string]any, error) {
	rows, err := q.db.DB().Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
