package build_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/build"
	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// --- Mocks ---

type mockMemWriter struct{}

func (m *mockMemWriter) Create(mem *domain.Memory) error    { return nil }
func (m *mockMemWriter) Update(mem *domain.Memory) error    { return nil }
func (m *mockMemWriter) Deactivate(id string) error         { return nil }
func (m *mockMemWriter) Touch(id string) error              { return nil }

type mockMemReader struct {
	memories []*domain.Memory
}

func (m *mockMemReader) Get(id string) (*domain.Memory, error) {
	for _, mem := range m.memories {
		if mem.ID == id {
			return mem, nil
		}
	}
	return nil, nil
}
func (m *mockMemReader) GetMany(ids []string) ([]*domain.Memory, error)                             { return nil, nil }
func (m *mockMemReader) QueryByFile(path string, limit, minImp int) ([]*domain.Memory, error)       { return nil, nil }
func (m *mockMemReader) Search(query string, opts domain.SearchOpts) ([]*domain.Memory, error)       { return nil, nil }
func (m *mockMemReader) Stats() (map[string]any, error)                                              { return map[string]any{}, nil }
func (m *mockMemReader) Count(active bool) (int, error)                                              { return len(m.memories), nil }
func (m *mockMemReader) ListAll(activeOnly bool, limit int) ([]*domain.Memory, error)                { return m.memories, nil }

type mockLinkStore struct{}

func (m *mockLinkStore) GetLinksFor(memID string) ([]*domain.MemoryLink, error) { return nil, nil }
func (m *mockLinkStore) GetLinkedIDs(memID string) ([]string, error)            { return nil, nil }
func (m *mockLinkStore) ListAll(limit int) ([]*domain.MemoryLink, error)        { return nil, nil }
func (m *mockLinkStore) CreateLink(link *domain.MemoryLink) error               { return nil }
func (m *mockLinkStore) DeleteForMemory(memID string) error                     { return nil }

type mockBuildMeta struct {
	last *domain.BuildMetaEntry
}

func (m *mockBuildMeta) Record(entry *domain.BuildMetaEntry) error     { m.last = entry; return nil }
func (m *mockBuildMeta) GetLast() (*domain.BuildMetaEntry, error)      { return m.last, nil }
func (m *mockBuildMeta) ListBuilds(limit int) ([]*domain.BuildMetaEntry, error) { return nil, nil }

type mockProcessed struct {
	data map[string]bool
}

func (m *mockProcessed) ReadProcessed() (map[string]bool, error) {
	if m.data == nil {
		m.data = make(map[string]bool)
	}
	return m.data, nil
}
func (m *mockProcessed) AddProcessed(p map[string]bool) error {
	if m.data == nil {
		m.data = make(map[string]bool)
	}
	for k, v := range p {
		m.data[k] = v
	}
	return nil
}
func (m *mockProcessed) ClearProcessed() error { m.data = make(map[string]bool); return nil }

type mockJSONStore struct {
	memories []*domain.Memory
	written  []*domain.Memory
}

func (m *mockJSONStore) Write(mem *domain.Memory) error {
	m.written = append(m.written, mem)
	m.memories = append(m.memories, mem)
	return nil
}
func (m *mockJSONStore) Read(id string) (*domain.Memory, error) {
	for _, mem := range m.memories {
		if mem.ID == id {
			return mem, nil
		}
	}
	return nil, nil
}
func (m *mockJSONStore) ReadAll() ([]*domain.Memory, error) { return m.memories, nil }
func (m *mockJSONStore) Delete(id string) (bool, error) {
	for i, mem := range m.memories {
		if mem.ID == id {
			m.memories = append(m.memories[:i], m.memories[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}
func (m *mockJSONStore) ComputeFingerprint() (string, error) { return "fp", nil }

type mockGitParser struct {
	hashes  []string
	commits []*domain.ParsedCommit
}

func (m *mockGitParser) GetAllHashes(limit int) ([]string, error) { return m.hashes, nil }
func (m *mockGitParser) GetCommitsByHashes(hashes []string) ([]*domain.ParsedCommit, error) {
	if m.commits != nil {
		return m.commits, nil
	}
	var result []*domain.ParsedCommit
	for _, h := range hashes {
		result = append(result, &domain.ParsedCommit{Hash: h, Message: "test", Files: []string{"test.go"}, Diff: "diff"})
	}
	return result, nil
}
func (m *mockGitParser) GetCurrentHash() (string, error) { return "abc123", nil }

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Chat(msgs []domain.Message, opts domain.ChatOpts) (string, error) {
	return m.response, m.err
}
func (m *mockLLM) GetModelInfo() (domain.ModelInfo, error) {
	return domain.ModelInfo{ID: "mock"}, nil
}

// --- Helper ---

func newAgent(processed *mockProcessed, gitParser *mockGitParser, jsonStore *mockJSONStore, llm *mockLLM) *build.Agent {
	cfg := config.Load()
	return build.NewAgent(
		&mockMemWriter{}, &mockMemReader{}, &mockLinkStore{},
		&mockBuildMeta{}, processed, jsonStore,
		gitParser, llm, cfg,
	)
}

// --- Tests ---

func TestAgent_Run_NoUnprocessed(t *testing.T) {
	processed := &mockProcessed{data: map[string]bool{"abc": true, "def": true}}
	gitParser := &mockGitParser{hashes: []string{"abc", "def"}}

	agent := newAgent(processed, gitParser, &mockJSONStore{}, &mockLLM{})
	err := agent.Run(context.Background(), 0, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestAgent_Run_WithExtraction(t *testing.T) {
	processed := &mockProcessed{}
	extractionJSON := `{"memories":[{"summary":"Test memory","type":"decision","importance":50,"source_commits":["abc"],"file_paths":["test.go"],"tags":["test"]}]}`
	gitParser := &mockGitParser{
		hashes: []string{"abc"},
		commits: []*domain.ParsedCommit{
			{Hash: "abc", Message: "test commit", Files: []string{"test.go"}, Diff: "+added"},
		},
	}
	jsonStore := &mockJSONStore{}
	agent := newAgent(processed, gitParser, jsonStore, &mockLLM{response: extractionJSON})

	err := agent.Run(context.Background(), 0, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	p, _ := processed.ReadProcessed()
	if !p["abc"] {
		t.Error("expected 'abc' to be marked as processed")
	}
	if len(jsonStore.written) == 0 {
		t.Error("expected at least one memory to be written")
	}
}

func TestAgent_Run_AllFiltered(t *testing.T) {
	processed := &mockProcessed{}
	gitParser := &mockGitParser{
		hashes: []string{"abc"},
		commits: []*domain.ParsedCommit{
			{Hash: "abc", Message: "data update", Files: []string{".agent/mcp/mcp-project-memory/data/foo.json"}},
		},
	}

	agent := newAgent(processed, gitParser, &mockJSONStore{}, &mockLLM{})
	err := agent.Run(context.Background(), 0, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	p, _ := processed.ReadProcessed()
	if !p["abc"] {
		t.Error("expected filtered commit to be marked as processed")
	}
}

func TestAgent_Run_WithLimit(t *testing.T) {
	processed := &mockProcessed{}
	extractionJSON := `{"memories":[{"summary":"Test","type":"decision","importance":50,"source_commits":["a"],"file_paths":["test.go"],"tags":["t"]}]}`
	gitParser := &mockGitParser{
		hashes: []string{"a", "b", "c"},
		commits: []*domain.ParsedCommit{
			{Hash: "a", Message: "a", Files: []string{"a.go"}, Diff: "+a"},
		},
	}

	agent := newAgent(processed, gitParser, &mockJSONStore{}, &mockLLM{response: extractionJSON})
	err := agent.Run(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	p, _ := processed.ReadProcessed()
	if len(p) != 1 {
		t.Errorf("expected 1 processed, got %d", len(p))
	}
}

func TestAgent_Run_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gitParser := &mockGitParser{hashes: []string{"abc"}}
	agent := newAgent(&mockProcessed{}, gitParser, &mockJSONStore{}, &mockLLM{})

	err := agent.Run(ctx, 0, false)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestFormatCommitsForExtraction_WithTrailersAndBody(t *testing.T) {
	commits := []*domain.ParsedCommit{
		{
			Hash: "abc123", Author: "User", Date: "2025-01-01",
			Message: "Add feature", Body: "Detailed body",
			Files: []string{"main.go", "utils.go"},
			Trailers: map[string]string{"Signed-off-by": "User"},
			Diff: "+added line",
		},
	}

	result := build.FormatCommitsForExtraction(commits)
	if !strings.Contains(result, "abc123") {
		t.Error("expected commit hash in output")
	}
	if !strings.Contains(result, "Detailed body") {
		t.Error("expected body in output")
	}
	if !strings.Contains(result, "Signed-off-by") {
		t.Error("expected trailers in output")
	}
}
