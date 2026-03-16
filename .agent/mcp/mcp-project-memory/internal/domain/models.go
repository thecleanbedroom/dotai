// Package domain defines the core data structures and interfaces for the
// project-memory system. This package has ZERO external imports — all
// infrastructure depends inward on these types.
package domain

import (
	"context"
	"encoding/json"
	"time"
)

// --- Constants ---

// MemoryTypes enumerates valid memory type values.
var MemoryTypes = map[string]bool{
	"decision":     true,
	"pattern":      true,
	"convention":   true,
	"context":      true,
	"debt":         true,
	"bug_fix":      true,
	"refactor":     true,
	"fix":          true,
	"feature":      true,
	"architecture": true,
	"style":        true,
}

// RelationshipTypes enumerates valid link relationship values.
var RelationshipTypes = map[string]bool{
	"supersedes":     true,
	"contradicts":    true,
	"supports":       true,
	"extends":        true,
	"related_to":     true,
	"depends_on":     true,
	"caused_by":      true,
	"alternative_to": true,
}

// --- Memory ---

// Memory is the core domain entity — a single extracted insight from git history.
type Memory struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	Type          string   `json:"type"`
	Confidence    int      `json:"confidence"`
	Importance    int      `json:"importance"`
	SourceCommits []string `json:"source_commits"`
	FilePaths     []string `json:"file_paths"`
	Tags          []string `json:"tags"`
	CreatedAt     string   `json:"created_at"`
	AccessedAt    string   `json:"accessed_at,omitempty"`
	AccessCount   int      `json:"access_count"`
	Active        bool     `json:"active"`
	// Links is populated from the DB at query time, not persisted in the memories table.
	Links []map[string]any `json:"links,omitempty"`
}

// ToDict returns a representation suitable for MCP tool responses.
func (m *Memory) ToDict() map[string]any {
	d := map[string]any{
		"id":             m.ID,
		"summary":        m.Summary,
		"type":           m.Type,
		"confidence":     m.Confidence,
		"importance":     m.Importance,
		"source_commits": m.SourceCommits,
		"file_paths":     m.FilePaths,
		"tags":           m.Tags,
		"created_at":     m.CreatedAt,
	}
	if m.AccessedAt != "" {
		d["accessed_at"] = m.AccessedAt
	}
	if m.AccessCount > 0 {
		d["access_count"] = m.AccessCount
	}
	if len(m.Links) > 0 {
		d["links"] = m.Links
	}
	return d
}

// ToJSONDict returns the full representation for on-disk JSON files.
func (m *Memory) ToJSONDict() map[string]any {
	d := map[string]any{
		"id":             m.ID,
		"summary":        m.Summary,
		"type":           m.Type,
		"confidence":     m.Confidence,
		"importance":     m.Importance,
		"source_commits": m.SourceCommits,
		"file_paths":     m.FilePaths,
		"tags":           m.Tags,
		"created_at":     m.CreatedAt,
		"accessed_at":    m.AccessedAt,
		"access_count":   m.AccessCount,
		"active":         m.Active,
	}
	if len(m.Links) > 0 {
		d["links"] = m.Links
	}
	return d
}

// FromJSONDict populates a Memory from a map (read from a JSON file).
func FromJSONDict(data map[string]any) *Memory {
	m := &Memory{
		Active: true, // default
	}

	if v, ok := data["id"].(string); ok {
		m.ID = v
	}
	if v, ok := data["summary"].(string); ok {
		m.Summary = v
	}
	if v, ok := data["type"].(string); ok {
		m.Type = v
	}
	if v, ok := data["confidence"].(float64); ok {
		m.Confidence = int(v)
	}
	if v, ok := data["importance"].(float64); ok {
		m.Importance = int(v)
	}
	if v, ok := data["source_commits"].([]any); ok {
		m.SourceCommits = toStringSlice(v)
	}
	if v, ok := data["file_paths"].([]any); ok {
		m.FilePaths = toStringSlice(v)
	}
	if v, ok := data["tags"].([]any); ok {
		m.Tags = toStringSlice(v)
	}
	if v, ok := data["created_at"].(string); ok {
		m.CreatedAt = v
	}
	if v, ok := data["accessed_at"].(string); ok {
		m.AccessedAt = v
	}
	if v, ok := data["access_count"].(float64); ok {
		m.AccessCount = int(v)
	}
	if v, ok := data["active"].(bool); ok {
		m.Active = v
	}
	if v, ok := data["links"].([]any); ok {
		for _, item := range v {
			if linkMap, ok := item.(map[string]any); ok {
				m.Links = append(m.Links, linkMap)
			}
		}
	}

	// Ensure non-nil slices for JSON serialization
	if m.SourceCommits == nil {
		m.SourceCommits = []string{}
	}
	if m.FilePaths == nil {
		m.FilePaths = []string{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}

	return m
}

func toStringSlice(items []any) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// --- MemoryLink ---

// MemoryLink is a directional relationship between two memories.
type MemoryLink struct {
	ID           int     `json:"id,omitempty"`
	MemoryIDA    string  `json:"memory_id_a"`
	MemoryIDB    string  `json:"memory_id_b"`
	Relationship string  `json:"relationship"`
	Strength     float64 `json:"strength"`
	CreatedAt    string  `json:"created_at,omitempty"`
}

// --- BuildMetaEntry ---

// BuildMetaEntry records metadata about a build run.
type BuildMetaEntry struct {
	ID          int    `json:"id,omitempty"`
	BuildType   string `json:"build_type"`
	CommitCount int    `json:"commit_count"`
	MemoryCount int    `json:"memory_count"`
	BuiltAt     string `json:"built_at,omitempty"`
}

// --- ParsedCommit ---

// ParsedCommit represents a single git commit parsed for LLM consumption.
type ParsedCommit struct {
	Hash     string            `json:"hash"`
	Author   string            `json:"author"`
	Date     string            `json:"date"`
	Message  string            `json:"message"`
	Body     string            `json:"body,omitempty"`
	Diff     string            `json:"diff,omitempty"`
	Files    []string          `json:"files,omitempty"`
	Trailers map[string]string `json:"trailers,omitempty"`
}

// --- Search options ---

// SearchOpts holds parameters for FTS5 search.
type SearchOpts struct {
	Type         string
	Match        string // "any" or "all"
	MinImportance int
	Limit        int
	Since        string
	Until        string
	ExcludeTags  []string
}

// --- LLM types ---

// Message is a chat message for LLM calls.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOpts holds parameters for an LLM chat call.
type ChatOpts struct {
	Ctx            context.Context `json:"-"`
	MaxTokens      int
	ResponseSchema json.RawMessage
	Label          string
	ThinkingBudget int
}

// ModelInfo holds information about an LLM model.
type ModelInfo struct {
	ID                  string             `json:"id"`
	ContextLength       int                `json:"context_length"`
	MaxCompletionTokens int                `json:"max_completion_tokens"`
	Name                string             `json:"name"`
	SupportedParams     []string           `json:"supported_parameters"`
	Pricing             map[string]float64 `json:"pricing"`
	IsFree              bool               `json:"is_free"`
}

// --- Time helper ---

// NowUTC returns the current time in UTC ISO format.
func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
