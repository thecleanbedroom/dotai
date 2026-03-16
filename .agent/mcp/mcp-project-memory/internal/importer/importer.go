// Package importer provides functionality for importing project memories from external files.
package importer

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/google/uuid"
)

// MemoryStorer defines the interface required for storing and retrieving memories.
// This is satisfied by *storage.MemoryStore.
type MemoryStorer interface {
	Get(id string) (*domain.Memory, error)
	Create(m *domain.Memory) error
	Update(m *domain.Memory) error
}

// Importer handles importing memories from an external source.
type Importer struct {
	store MemoryStorer
}

// NewImporter creates a new Importer with the given memory store.
func NewImporter(store MemoryStorer) *Importer {
	return &Importer{
		store: store,
	}
}

// ImportFile imports memories from a JSON file at a given path.
// It performs an "upsert":
// - If a memory from the file has an ID that already exists in the store, it's updated.
// - Otherwise, the memory is created as a new entry.
// - If a memory in the file is missing an ID, a new UUID is generated for it before creation.
// It returns the number of memories created and updated, respectively.
func (i *Importer) ImportFile(filePath string) (createdCount int, updatedCount int, err error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("importer: failed to read file '%s': %w", filePath, err)
	}

	var memories []*domain.Memory
	if err := json.Unmarshal(data, &memories); err != nil {
		return 0, 0, fmt.Errorf("importer: failed to unmarshal memories from '%s': %w", filePath, err)
	}

	for _, m := range memories {
		if m.ID == "" {
			m.ID = uuid.NewString()
		}

		existing, err := i.store.Get(m.ID)
		if err != nil {
			return createdCount, updatedCount, fmt.Errorf("importer: failed checking for memory '%s': %w", m.ID, err)
		}

		if existing != nil {
			// Update existing memory
			m.CreatedAt = existing.CreatedAt // Preserve original creation timestamp
			if m.AccessedAt == "" {
				m.AccessedAt = existing.AccessedAt
			}
			if err := i.store.Update(m); err != nil {
				return createdCount, updatedCount, fmt.Errorf("importer: failed to update memory '%s': %w", m.ID, err)
			}
			updatedCount++
		} else {
			// Create new memory
			if m.CreatedAt == "" {
				m.CreatedAt = domain.NowUTC()
			}
			if err := i.store.Create(m); err != nil {
				return createdCount, updatedCount, fmt.Errorf("importer: failed to create memory '%s': %w", m.ID, err)
			}
			createdCount++
		}
	}

	return createdCount, updatedCount, nil
}
