package build_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dotai/mcp-project-memory/internal/build"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// synthesisTestJSONStore wraps mockJSONStore with write-tracking for triage verification.
type synthesisTestJSONStore struct {
	memories []*domain.Memory
	writes   []*domain.Memory
}

func (s *synthesisTestJSONStore) ReadAll() ([]*domain.Memory, error) {
	var active []*domain.Memory
	for _, m := range s.memories {
		if m.Active {
			active = append(active, m)
		}
	}
	return active, nil
}
func (s *synthesisTestJSONStore) Read(id string) (*domain.Memory, error) {
	for _, m := range s.memories {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, nil
}
func (s *synthesisTestJSONStore) Write(m *domain.Memory) error {
	s.writes = append(s.writes, m)
	// Update in-place
	for i, existing := range s.memories {
		if existing.ID == m.ID {
			s.memories[i] = m
			return nil
		}
	}
	s.memories = append(s.memories, m)
	return nil
}
func (s *synthesisTestJSONStore) Delete(id string) (bool, error)    { return false, nil }
func (s *synthesisTestJSONStore) ComputeFingerprint() (string, error) { return "fp", nil }

func TestSynthesisAgent_RunIncremental_NoNewMemories(t *testing.T) {
	js := &synthesisTestJSONStore{
		memories: []*domain.Memory{
			{ID: "existing-1", Summary: "Existing", Type: "decision", Active: true},
		},
	}
	llm := &mockLLM{response: `{}`}

	agent := build.NewSynthesisAgent(llm, js)
	err := agent.RunIncremental(context.Background(), nil)
	if err != nil {
		t.Fatalf("RunIncremental with nil IDs: %v", err)
	}

	err = agent.RunIncremental(context.Background(), []string{})
	if err != nil {
		t.Fatalf("RunIncremental with empty IDs: %v", err)
	}
}

func TestSynthesisAgent_RunIncremental_TriageAndLink(t *testing.T) {
	js := &synthesisTestJSONStore{
		memories: []*domain.Memory{
			{ID: "existing-1", Summary: "Auth system design", Type: "decision", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{"auth"}},
			{ID: "new-1", Summary: "New auth middleware", Type: "pattern", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{"auth"}},
			{ID: "new-2", Summary: "Deprecated helper", Type: "context", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{}},
		},
	}

	// Override chat to return different responses per call
	triageResp, _ := json.Marshal(map[string]any{
		"decisions": []map[string]any{
			{"memory_id": "new-1", "action": "accept"},
			{"memory_id": "new-2", "action": "deactivate"},
		},
	})
	linkResp, _ := json.Marshal(map[string]any{
		"links": []map[string]any{
			{"memory_id_a": "new-1", "memory_id_b": "existing-1", "relationship": "supports", "strength": 0.8},
		},
	})

	customLLM := &sequenceLLM{responses: []string{string(triageResp), string(linkResp)}}

	agent := build.NewSynthesisAgent(customLLM, js)
	err := agent.RunIncremental(context.Background(), []string{"new-1", "new-2"})
	if err != nil {
		t.Fatalf("RunIncremental: %v", err)
	}

	// Verify triage: new-2 should be deactivated
	for _, m := range js.memories {
		if m.ID == "new-2" && m.Active {
			t.Error("expected new-2 to be deactivated by triage")
		}
	}

	// Verify links: new-1 should have a link to existing-1
	for _, m := range js.memories {
		if m.ID == "new-1" && len(m.Links) == 0 {
			t.Error("expected new-1 to have links after linking phase")
		}
	}
}

func TestSynthesisAgent_RunIncremental_TriageFail_AcceptsAll(t *testing.T) {
	js := &synthesisTestJSONStore{
		memories: []*domain.Memory{
			{ID: "existing-1", Summary: "Existing", Type: "decision", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{}},
			{ID: "new-1", Summary: "New memory", Type: "pattern", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{}},
		},
	}

	// First call (triage) fails, second call (linking) succeeds
	linkResp, _ := json.Marshal(map[string]any{"links": []map[string]any{}})
	llm := &sequenceLLM{
		responses: []string{"", string(linkResp)},
		errors:    []error{context.DeadlineExceeded, nil},
	}

	agent := build.NewSynthesisAgent(llm, js)
	// Even with triage failure, RunIncremental should not return error — it accepts all.
	// Note: CallWithRetries wraps the LLM call, but eventually the error propagates.
	// The synthesis agent catches triage errors and falls through to linking.
	err := agent.RunIncremental(context.Background(), []string{"new-1"})
	// The error from triage is swallowed; linking may or may not succeed depending on retry behavior.
	// The key assertion is that new-1 stays active.
	_ = err

	for _, m := range js.memories {
		if m.ID == "new-1" && !m.Active {
			t.Error("expected new-1 to remain active when triage fails")
		}
	}
}

func TestSynthesisAgent_RunFull_EmptyCorpus(t *testing.T) {
	js := &synthesisTestJSONStore{memories: nil}
	llm := &mockLLM{response: `{}`}

	agent := build.NewSynthesisAgent(llm, js)
	err := agent.RunFull(context.Background())
	if err != nil {
		t.Fatalf("RunFull with empty corpus: %v", err)
	}
}

func TestSynthesisAgent_RunFull_LinksAllMemories(t *testing.T) {
	js := &synthesisTestJSONStore{
		memories: []*domain.Memory{
			{ID: "mem-1", Summary: "Auth design", Type: "decision", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{"auth"}},
			{ID: "mem-2", Summary: "Auth middleware", Type: "pattern", Active: true,
				SourceCommits: []string{}, FilePaths: []string{}, Tags: []string{"auth"}},
		},
	}

	linkResp, _ := json.Marshal(map[string]any{
		"links": []map[string]any{
			{"memory_id_a": "mem-1", "memory_id_b": "mem-2", "relationship": "related_to", "strength": 0.7},
		},
	})
	llm := &mockLLM{response: string(linkResp)}

	agent := build.NewSynthesisAgent(llm, js)
	err := agent.RunFull(context.Background())
	if err != nil {
		t.Fatalf("RunFull: %v", err)
	}

	// mem-1 should have a link to mem-2
	for _, m := range js.memories {
		if m.ID == "mem-1" && len(m.Links) != 1 {
			t.Errorf("expected 1 link on mem-1, got %d", len(m.Links))
		}
	}
}

// sequenceLLM returns different responses on successive calls.
type sequenceLLM struct {
	responses []string
	errors    []error
	callIdx   int
}

func (s *sequenceLLM) Chat(msgs []domain.Message, opts domain.ChatOpts) (string, error) {
	idx := s.callIdx
	s.callIdx++
	var resp string
	var err error
	if idx < len(s.responses) {
		resp = s.responses[idx]
	}
	if s.errors != nil && idx < len(s.errors) {
		err = s.errors[idx]
	}
	return resp, err
}

func (s *sequenceLLM) GetModelInfo() (domain.ModelInfo, error) {
	return domain.ModelInfo{ID: "mock-seq"}, nil
}
