package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/domain"
)

func TestMemory_ToDict(t *testing.T) {
	m := &domain.Memory{
		ID:            "test-uuid",
		Summary:       "Test summary",
		Type:          "decision",
		Confidence:    75,
		Importance:    90,
		SourceCommits: []string{"abc123"},
		FilePaths:     []string{"foo.go"},
		Tags:          []string{"arch"},
		CreatedAt:     "2025-01-01T00:00:00Z",
		AccessedAt:    "2025-01-02T00:00:00Z",
		AccessCount:   3,
		Active:        true,
	}

	d := m.ToDict()
	if d["id"] != "test-uuid" {
		t.Errorf("id: got %v", d["id"])
	}
	if d["summary"] != "Test summary" {
		t.Errorf("summary: got %v", d["summary"])
	}
	if d["type"] != "decision" {
		t.Errorf("type: got %v", d["type"])
	}
	if d["confidence"].(int) != 75 {
		t.Errorf("confidence: got %v", d["confidence"])
	}
	if d["importance"].(int) != 90 {
		t.Errorf("importance: got %v", d["importance"])
	}
}

func TestMemory_ToJSONDict(t *testing.T) {
	m := &domain.Memory{
		ID:            "uuid-1",
		Summary:       "JSON test",
		Type:          "pattern",
		SourceCommits: []string{"abc"},
		FilePaths:     []string{"bar.go"},
		Tags:          []string{"tag1", "tag2"},
		CreatedAt:     "2025-01-01T00:00:00Z",
		Active:        true,
	}

	d := m.ToJSONDict()
	// Should have all expected keys
	for _, key := range []string{"id", "summary", "type", "confidence", "importance", "source_commits", "file_paths", "tags", "created_at", "active"} {
		if _, ok := d[key]; !ok {
			t.Errorf("missing key %q in ToJSONDict", key)
		}
	}
	if d["active"] != true {
		t.Errorf("active: got %v", d["active"])
	}
}

func TestMemory_MarshalJSON(t *testing.T) {
	m := &domain.Memory{
		ID:            "json-marshal-id",
		Summary:       "Marshal test",
		Type:          "convention",
		SourceCommits: []string{"def456"},
		FilePaths:     []string{},
		Tags:          []string{},
		CreatedAt:     "2025-06-01T00:00:00Z",
		Active:        true,
	}

	data, err := json.Marshal(m.ToJSONDict())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), "json-marshal-id") {
		t.Error("expected JSON to contain memory ID")
	}
}

func TestFromJSONDict(t *testing.T) {
	d := map[string]any{
		"id":             "from-json-id",
		"summary":        "From JSON",
		"type":           "debt",
		"confidence":     float64(40),
		"importance":     float64(60),
		"source_commits": []any{"abc", "def"},
		"file_paths":     []any{"a.go", "b.go"},
		"tags":           []any{"t1"},
		"created_at":     "2025-01-01T00:00:00Z",
		"accessed_at":    "2025-01-02T00:00:00Z",
		"access_count":   float64(5),
		"active":         true,
	}

	m := domain.FromJSONDict(d)
	if m.ID != "from-json-id" {
		t.Errorf("ID: got %q", m.ID)
	}
	if m.Summary != "From JSON" {
		t.Errorf("Summary: got %q", m.Summary)
	}
	if m.Type != "debt" {
		t.Errorf("Type: got %q", m.Type)
	}
	if m.Confidence != 40 {
		t.Errorf("Confidence: got %d", m.Confidence)
	}
	if len(m.SourceCommits) != 2 {
		t.Errorf("SourceCommits: got %v", m.SourceCommits)
	}
	if m.AccessCount != 5 {
		t.Errorf("AccessCount: got %d", m.AccessCount)
	}
	if !m.Active {
		t.Error("expected Active=true")
	}
}

func TestFromJSONDict_Defaults(t *testing.T) {
	d := map[string]any{
		"id":      "minimal",
		"summary": "Minimal",
		"type":    "context",
	}

	m := domain.FromJSONDict(d)
	if m.ID != "minimal" {
		t.Errorf("ID: got %q", m.ID)
	}
	if m.SourceCommits == nil || len(m.SourceCommits) != 0 {
		t.Errorf("expected empty SourceCommits slice, got %v", m.SourceCommits)
	}
	if m.FilePaths == nil || len(m.FilePaths) != 0 {
		t.Errorf("expected empty FilePaths slice, got %v", m.FilePaths)
	}
	if m.Tags == nil || len(m.Tags) != 0 {
		t.Errorf("expected empty Tags slice, got %v", m.Tags)
	}
}

func TestFromJSONDict_WithLinks(t *testing.T) {
	d := map[string]any{
		"id":      "linked",
		"summary": "Linked",
		"type":    "decision",
		"links": []any{
			map[string]any{"target": "other-id", "relationship": "supports", "strength": 0.8},
		},
	}

	m := domain.FromJSONDict(d)
	if len(m.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(m.Links))
	}
	if m.Links[0]["target"] != "other-id" {
		t.Errorf("link target: got %v", m.Links[0]["target"])
	}
}

func TestNowUTC(t *testing.T) {
	ts := domain.NowUTC()
	if ts == "" {
		t.Error("NowUTC returned empty string")
	}
	if !strings.Contains(ts, "T") {
		t.Errorf("NowUTC doesn't look like ISO format: %q", ts)
	}
}

func TestMemoryTypes(t *testing.T) {
	expected := []string{"decision", "pattern", "convention", "context", "debt"}
	for _, mt := range expected {
		if !domain.MemoryTypes[mt] {
			t.Errorf("expected MemoryTypes[%q] to be true", mt)
		}
	}
	if domain.MemoryTypes["invalid"] {
		t.Error("MemoryTypes should not contain 'invalid'")
	}
}

func TestSearchOpts_Defaults(t *testing.T) {
	opts := domain.SearchOpts{}
	if opts.Limit != 0 {
		t.Errorf("default Limit should be 0, got %d", opts.Limit)
	}
	if opts.Match != "" {
		t.Errorf("default Match should be empty, got %q", opts.Match)
	}
}

func TestParsedCommit(t *testing.T) {
	c := &domain.ParsedCommit{
		Hash:     "abcdef123456",
		Author:   "Test Author",
		Date:     "2025-01-01",
		Message:  "test commit",
		Body:     "body text",
		Diff:     "+added line",
		Files:    []string{"foo.go"},
		Trailers: map[string]string{"Signed-off-by": "test"},
	}

	if c.Hash != "abcdef123456" {
		t.Errorf("Hash: got %q", c.Hash)
	}
	if len(c.Files) != 1 {
		t.Errorf("Files: got %v", c.Files)
	}
	if c.Trailers["Signed-off-by"] != "test" {
		t.Errorf("Trailers: got %v", c.Trailers)
	}
}

func TestModelInfo(t *testing.T) {
	info := domain.ModelInfo{
		Name:          "test-model",
		ContextLength: 128000,
		IsFree:        true,
	}
	if info.Name != "test-model" {
		t.Errorf("Name: got %q", info.Name)
	}
	if info.ContextLength != 128000 {
		t.Errorf("ContextLength: got %d", info.ContextLength)
	}
}
