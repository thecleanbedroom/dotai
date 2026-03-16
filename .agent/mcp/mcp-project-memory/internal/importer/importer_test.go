package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/google/go-cmp/cmp"
)

// --- Mock Store ---

type mockMemoryStore struct {
	memories map[string]*domain.Memory
}

func newMockStore() *mockMemoryStore {
	return &mockMemoryStore{
		memories: make(map[string]*domain.Memory),
	}
}

func (s *mockMemoryStore) Get(id string) (*domain.Memory, error) {
	m, ok := s.memories[id]
	if !ok {
		return nil, nil
	}
	return m, nil
}

func (s *mockMemoryStore) Create(m *domain.Memory) error {
	if _, exists := s.memories[m.ID]; exists {
		return fmt.Errorf("mock: memory with ID '%s' already exists", m.ID)
	}
	s.memories[m.ID] = m
	return nil
}

func (s *mockMemoryStore) Update(m *domain.Memory) error {
	if _, exists := s.memories[m.ID]; !exists {
		return fmt.Errorf("mock: memory with ID '%s' not found", m.ID)
	}
	s.memories[m.ID] = m
	return nil
}

// --- Tests ---

func TestImportFile(t *testing.T) {
	t.Run("creates new memories and updates existing ones", func(t *testing.T) {
		// 1. Setup
		store := newMockStore()
		importer := NewImporter(store)
		tempDir := t.TempDir()

		// Pre-populate store with one memory to be updated
		existingMemory := &domain.Memory{
			ID:        "existing-uuid",
			Summary:   "Original summary",
			CreatedAt: time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339),
		}
		store.memories[existingMemory.ID] = existingMemory

		// Prepare JSON file for import
		importMemories := []*domain.Memory{
			{
				ID:      "existing-uuid",
				Summary: "Updated summary",
			},
			{
				// This one has no ID, so a new one should be generated
				Summary: "A brand new memory",
				Type:    "decision",
			},
		}
		jsonData, _ := json.Marshal(importMemories)
		jsonPath := filepath.Join(tempDir, "import.json")
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			t.Fatalf("Failed to write temp import file: %v", err)
		}

		// 2. Execute
		created, updated, err := importer.ImportFile(jsonPath)

		// 3. Assert
		if err != nil {
			t.Fatalf("ImportFile failed: %v", err)
		}
		if created != 1 {
			t.Errorf("Expected 1 memory created, got %d", created)
		}
		if updated != 1 {
			t.Errorf("Expected 1 memory updated, got %d", updated)
		}
		if len(store.memories) != 2 {
			t.Fatalf("Expected total of 2 memories in store, got %d", len(store.memories))
		}

		// Assert updated memory
		updatedMem := store.memories["existing-uuid"]
		if updatedMem.Summary != "Updated summary" {
			t.Errorf("Expected summary to be updated, but it was '%s'", updatedMem.Summary)
		}
		if updatedMem.CreatedAt != existingMemory.CreatedAt {
			t.Errorf("Expected CreatedAt to be preserved, but it changed")
		}

		// Assert created memory
		var newMem *domain.Memory
		for id, mem := range store.memories {
			if id != "existing-uuid" {
				newMem = mem
				break
			}
		}
		if newMem == nil {
			t.Fatal("Could not find the new memory in the store")
		}
		if newMem.Summary != "A brand new memory" {
			t.Errorf("New memory has wrong summary: '%s'", newMem.Summary)
		}
		if newMem.ID == "" {
			t.Error("New memory was not assigned an ID")
		}
		if newMem.CreatedAt == "" {
			t.Error("New memory was not assigned a CreatedAt timestamp")
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		store := newMockStore()
		importer := NewImporter(store)

		_, _, err := importer.ImportFile("non-existent-file.json")

		if err == nil {
			t.Fatal("Expected an error for a non-existent file, but got nil")
		}
	})

	t.Run("returns error for malformed json", func(t *testing.T) {
		store := newMockStore()
		importer := NewImporter(store)
		tempDir := t.TempDir()

		jsonPath := filepath.Join(tempDir, "bad.json")
		if err := os.WriteFile(jsonPath, []byte("[{]"), 0644); err != nil {
			t.Fatalf("Failed to write temp import file: %v", err)
		}

		_, _, err := importer.ImportFile(jsonPath)

		if err == nil {
			t.Fatal("Expected an error for malformed JSON, but got nil")
		}
	})

	t.Run("preserves access time on update if not provided", func(t *testing.T) {
		store := newMockStore()
		importer := NewImporter(store)
		tempDir := t.TempDir()

		accessTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
		existingMemory := &domain.Memory{
			ID:         "abc",
			Summary:    "Old",
			AccessedAt: accessTime,
		}
		store.memories["abc"] = existingMemory

		importMemories := []*domain.Memory{{ID: "abc", Summary: "New"}}
		jsonData, _ := json.Marshal(importMemories)
		jsonPath := filepath.Join(tempDir, "import.json")
		os.WriteFile(jsonPath, jsonData, 0644)

		_, _, err := importer.ImportFile(jsonPath)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		updatedMem := store.memories["abc"]
		if diff := cmp.Diff(accessTime, updatedMem.AccessedAt); diff != "" {
			t.Errorf("AccessedAt was not preserved (-want +got):\n%s", diff)
		}
	})
}
