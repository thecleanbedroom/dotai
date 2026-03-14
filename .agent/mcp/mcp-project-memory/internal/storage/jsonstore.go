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

// FileJSONStore implements domain.JSONStore — reads/writes Memory JSON files
// and the processed.json commit tracker.
type FileJSONStore struct{}

// NewJSONStore creates a FileJSONStore.
func NewJSONStore() *FileJSONStore { return &FileJSONStore{} }

func memoriesDir(dataDir string) string {
	d := filepath.Join(dataDir, "memories")
	os.MkdirAll(d, 0o755)
	return d
}

func processedPath(dataDir string) string {
	return filepath.Join(dataDir, "processed.json")
}

// Write writes a memory to data/memories/{uuid}.json.
func (s *FileJSONStore) Write(m *domain.Memory, dataDir string) error {
	path := filepath.Join(memoriesDir(dataDir), m.ID+".json")
	data, err := json.MarshalIndent(m.ToJSONDict(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Read reads a single memory by UUID.
func (s *FileJSONStore) Read(id, dataDir string) (*domain.Memory, error) {
	path := filepath.Join(memoriesDir(dataDir), id+".json")
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
func (s *FileJSONStore) ReadAll(dataDir string) ([]*domain.Memory, error) {
	dir := memoriesDir(dataDir)
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
func (s *FileJSONStore) Delete(id, dataDir string) (bool, error) {
	path := filepath.Join(memoriesDir(dataDir), id+".json")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// ComputeFingerprint hashes sorted filenames + mtime/size for stale detection.
func (s *FileJSONStore) ComputeFingerprint(dataDir string) (string, error) {
	dir := memoriesDir(dataDir)
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

// ReadProcessed reads the set of already-processed commit hashes.
func (s *FileJSONStore) ReadProcessed(dataDir string) (map[string]bool, error) {
	raw, err := os.ReadFile(processedPath(dataDir))
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var hashes []string
	if err := json.Unmarshal(raw, &hashes); err != nil {
		return map[string]bool{}, nil
	}
	result := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		result[h] = true
	}
	return result, nil
}

// AddProcessed merges new hashes into processed.json.
func (s *FileJSONStore) AddProcessed(hashes map[string]bool, dataDir string) error {
	existing, err := s.ReadProcessed(dataDir)
	if err != nil {
		return err
	}
	for h := range hashes {
		existing[h] = true
	}
	var sorted []string
	for h := range existing {
		sorted = append(sorted, h)
	}
	sort.Strings(sorted)
	data, _ := json.MarshalIndent(sorted, "", "  ")
	return os.WriteFile(processedPath(dataDir), append(data, '\n'), 0o644)
}
