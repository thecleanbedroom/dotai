package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

// FileJSONStore implements domain.JSONStore — reads/writes Memory JSON files.
// Paths are derived from the Paths struct.
type FileJSONStore struct {
	paths *Paths
}

// NewJSONStore creates a FileJSONStore bound to the given Paths.
func NewJSONStore(paths *Paths) *FileJSONStore {
	return &FileJSONStore{paths: paths}
}

// Write writes a memory to data/memories/{uuid}.json.
func (s *FileJSONStore) Write(m *domain.Memory) error {
	if err := os.MkdirAll(s.paths.MemoriesDir, 0o755); err != nil {
		return fmt.Errorf("ensure memories dir: %w", err)
	}
	path := filepath.Join(s.paths.MemoriesDir, m.ID+".json")
	data, err := json.MarshalIndent(m.ToJSONDict(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Read reads a single memory by UUID.
func (s *FileJSONStore) Read(id string) (*domain.Memory, error) {
	path := filepath.Join(s.paths.MemoriesDir, id+".json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return domain.FromJSONDict(data), nil
}

// ReadAll reads all active memory JSON files.
func (s *FileJSONStore) ReadAll() ([]*domain.Memory, error) {
	dir := s.paths.MemoriesDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var memories []*domain.Memory
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(raw, &data); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: skipping corrupt memory file %s: %v\n", e.Name(), err)
			continue
		}
		m := domain.FromJSONDict(data)
		if m.Active {
			memories = append(memories, m)
		}
	}
	return memories, nil
}

// Delete removes a memory's JSON file.
func (s *FileJSONStore) Delete(id string) (bool, error) {
	path := filepath.Join(s.paths.MemoriesDir, id+".json")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// ComputeFingerprint hashes sorted filenames + mtime/size for stale detection.
func (s *FileJSONStore) ComputeFingerprint() (string, error) {
	dir := s.paths.MemoriesDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	h := sha256.New()
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s:%d:%d\n", e.Name(), info.ModTime().UnixNano(), info.Size())
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16], nil
}
