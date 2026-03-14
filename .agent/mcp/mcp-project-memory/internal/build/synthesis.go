package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
)

// SynthesisAgent handles the two-phase synthesis pass: triage + linking.
type SynthesisAgent struct {
	llm     domain.LLMCaller
	json    domain.JSONStore
	dataDir string
}

// NewSynthesisAgent creates a synthesis agent.
func NewSynthesisAgent(llm domain.LLMCaller, jsonStore domain.JSONStore, dataDir string) *SynthesisAgent {
	return &SynthesisAgent{
		llm:     llm,
		json:    jsonStore,
		dataDir: dataDir,
	}
}

// Run executes the full synthesis pass.
func (s *SynthesisAgent) Run(ctx context.Context) error {
	// Read all memories
	memories, err := s.json.ReadAll(s.dataDir)
	if err != nil {
		return fmt.Errorf("read memories: %w", err)
	}
	if len(memories) == 0 {
		fmt.Fprintln(os.Stderr, "  no memories to synthesize")
		return nil
	}

	fmt.Fprintf(os.Stderr, "  synthesis: processing %d memories\n", len(memories))

	// Phase 1: Triage
	triageResult, err := s.triage(ctx, memories)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  triage failed: %v\n", err)
		// Continue with linking even if triage fails
	} else {
		if err := s.applyTriage(triageResult, memories); err != nil {
			fmt.Fprintf(os.Stderr, "  apply triage failed: %v\n", err)
		}
	}

	// Phase 2: Linking
	linkResult, err := s.link(ctx, memories)
	if err != nil {
		return fmt.Errorf("linking: %w", err)
	}

	return s.applyLinks(linkResult, memories)
}

func (s *SynthesisAgent) triage(ctx context.Context, memories []*domain.Memory) (map[string]any, error) {
	// Build corpus of existing memories
	corpus := buildCorpus(memories)

	result, err := CallWithRetries(s.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.SynthesisTriagePrompt()},
				{Role: "user", Content: corpus},
			},
			domain.ChatOpts{
				ResponseSchema: config.SynthesisTriageSchema(),
				Label:          "synthesis_triage",
			},
		)
	}, nil)
	if err != nil {
		return nil, err
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return nil, fmt.Errorf("parse triage result: %w", err)
	}
	return parsed, nil
}

func (s *SynthesisAgent) link(ctx context.Context, memories []*domain.Memory) (map[string]any, error) {
	corpus := buildCorpus(memories)

	result, err := CallWithRetries(s.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.SynthesisLinkingPrompt()},
				{Role: "user", Content: corpus},
			},
			domain.ChatOpts{
				ResponseSchema: config.SynthesisLinkingSchema(),
				Label:          "synthesis_linking",
			},
		)
	}, nil)
	if err != nil {
		return nil, err
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		return nil, fmt.Errorf("parse linking result: %w", err)
	}
	return parsed, nil
}

func (s *SynthesisAgent) applyTriage(result map[string]any, memories []*domain.Memory) error {
	decisions, ok := result["decisions"].([]any)
	if !ok {
		return nil
	}

	memByID := make(map[string]*domain.Memory)
	for _, m := range memories {
		memByID[m.ID] = m
	}

	for _, d := range decisions {
		dec, ok := d.(map[string]any)
		if !ok {
			continue
		}
		id, _ := dec["memory_id"].(string)
		action, _ := dec["action"].(string)

		switch action {
		case "deactivate":
			if m, ok := memByID[id]; ok {
				m.Active = false
				s.json.Write(m, s.dataDir)
			}
		case "accept":
			// No-op for now
		}
	}
	return nil
}

func (s *SynthesisAgent) applyLinks(result map[string]any, memories []*domain.Memory) error {
	links, ok := result["links"].([]any)
	if !ok {
		return nil
	}

	memByID := make(map[string]*domain.Memory)
	for _, m := range memories {
		memByID[m.ID] = m
	}

	for _, l := range links {
		link, ok := l.(map[string]any)
		if !ok {
			continue
		}
		idA, _ := link["memory_id_a"].(string)
		idB, _ := link["memory_id_b"].(string)
		rel, _ := link["relationship"].(string)
		strength, _ := link["strength"].(float64)

		if _, ok := memByID[idA]; !ok {
			continue
		}
		if _, ok := memByID[idB]; !ok {
			continue
		}

		// Add link to memory A's JSON
		memA := memByID[idA]
		memA.Links = append(memA.Links, map[string]any{
			"target":       idB,
			"relationship": rel,
			"strength":     strength,
		})
		s.json.Write(memA, s.dataDir)
	}

	return nil
}

func buildCorpus(memories []*domain.Memory) string {
	var result string
	for _, m := range memories {
		result += fmt.Sprintf("## Memory %s [%s] (importance: %d, confidence: %d)\n",
			m.ID, m.Type, m.Importance, m.Confidence)
		result += m.Summary + "\n"
		if len(m.Tags) > 0 {
			result += fmt.Sprintf("Tags: %v\n", m.Tags)
		}
		result += "\n"
	}
	return result
}
