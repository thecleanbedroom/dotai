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
	llm  domain.LLMCaller
	json domain.JSONStore
}

// NewSynthesisAgent creates a synthesis agent.
func NewSynthesisAgent(llm domain.LLMCaller, jsonStore domain.JSONStore) *SynthesisAgent {
	return &SynthesisAgent{
		llm:  llm,
		json: jsonStore,
	}
}

// RunIncremental runs synthesis on NEW memories against existing corpus.
// Phase 1 (Triage): accept/reject new memories.
// Phase 2 (Linking): find relationships between accepted new and existing.
func (s *SynthesisAgent) RunIncremental(ctx context.Context, newMemoryIDs []string) error {
	// Read all memories from disk
	allMemories, err := s.json.ReadAll()
	if err != nil {
		return fmt.Errorf("read memories: %w", err)
	}

	// Split into new vs existing
	newIDSet := make(map[string]bool, len(newMemoryIDs))
	for _, id := range newMemoryIDs {
		newIDSet[id] = true
	}

	var newMemories, existingMemories []*domain.Memory
	for _, m := range allMemories {
		if newIDSet[m.ID] {
			newMemories = append(newMemories, m)
		} else {
			existingMemories = append(existingMemories, m)
		}
	}

	if len(newMemories) == 0 {
		return nil
	}

	// Build compact corpus of existing memories for context
	existingCorpus := serializeCompact(existingMemories)

	// Phase 1: Triage
	fmt.Fprintf(os.Stderr, "    phase 1: triaging %d new memories against %d existing...\n",
		len(newMemories), len(existingMemories))

	triageResult, err := s.triage(ctx, newMemories, existingCorpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "    triage failed: %v (accepting all)\n", err)
		// On failure, accept everything so linking can still run
	} else if triageResult != nil {
		s.applyTriage(triageResult, newMemories)
	}

	// Reload in case triage deactivated some
	allMemories, err = s.json.ReadAll()
	if err != nil {
		return fmt.Errorf("re-read memories after triage: %w", err)
	}
	var acceptedNew []*domain.Memory
	for _, m := range allMemories {
		if newIDSet[m.ID] && m.Active {
			acceptedNew = append(acceptedNew, m)
		}
	}

	fmt.Fprintf(os.Stderr, "    phase 1: %d accepted, %d rejected\n",
		len(acceptedNew), len(newMemories)-len(acceptedNew))

	if len(acceptedNew) == 0 {
		return nil
	}

	// Phase 2: Linking — compare accepted new against full corpus
	fullCorpus := serializeCompact(allMemories)
	fmt.Fprintf(os.Stderr, "    phase 2: linking %d memories against corpus of %d...\n",
		len(acceptedNew), len(allMemories))

	linkResult, err := s.link(ctx, acceptedNew, fullCorpus)
	if err != nil {
		return fmt.Errorf("linking: %w", err)
	}

	return s.applyLinks(linkResult, allMemories)
}

// RunFull runs a full re-synthesis pass on ALL existing memories (--synthesis flag).
// Skips triage (all memories are established). Only runs linking.
func (s *SynthesisAgent) RunFull(ctx context.Context) error {
	allMemories, err := s.json.ReadAll()
	if err != nil {
		return fmt.Errorf("read memories: %w", err)
	}
	if len(allMemories) == 0 {
		fmt.Fprintln(os.Stderr, "  no memories to synthesize")
		return nil
	}

	fmt.Fprintf(os.Stderr, "  re-synthesis: relinking %d memories...\n", len(allMemories))

	corpus := serializeCompact(allMemories)
	linkResult, err := s.link(ctx, allMemories, corpus)
	if err != nil {
		return fmt.Errorf("linking: %w", err)
	}

	return s.applyLinks(linkResult, allMemories)
}

func (s *SynthesisAgent) triage(ctx context.Context, newMemories []*domain.Memory, existingCorpus string) (map[string]any, error) {
	newData := serializeFull(newMemories)

	userMsg := fmt.Sprintf("NEW memories (%d):\n```json\n%s\n```\n\nEXISTING corpus:\n```json\n%s\n```",
		len(newMemories), newData, existingCorpus)

	result, err := CallWithRetries(ctx, s.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.SynthesisTriagePrompt()},
				{Role: "user", Content: userMsg},
			},
			domain.ChatOpts{
				Ctx:            ctx,
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
		preview := result
		if len(preview) > 200 {
			preview = preview[:200] + "…"
		}
		return nil, fmt.Errorf("parse triage result: %w\nraw response: %s", err, preview)
	}
	return parsed, nil
}

func (s *SynthesisAgent) link(ctx context.Context, memories []*domain.Memory, corpus string) (map[string]any, error) {
	batchData := serializeFull(memories)

	userMsg := fmt.Sprintf("BATCH memories (%d):\n```json\n%s\n```\n\nCORPUS:\n```json\n%s\n```",
		len(memories), batchData, corpus)

	result, err := CallWithRetries(ctx, s.llm, func(caller domain.LLMCaller) (string, error) {
		return caller.Chat(
			[]domain.Message{
				{Role: "system", Content: config.SynthesisLinkingPrompt()},
				{Role: "user", Content: userMsg},
			},
			domain.ChatOpts{
				Ctx:            ctx,
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

func (s *SynthesisAgent) applyTriage(result map[string]any, memories []*domain.Memory) {
	decisions, ok := result["decisions"].([]any)
	if !ok {
		return
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
		case "deactivate", "reject":
			if m, ok := memByID[id]; ok {
				m.Active = false
				if err := s.json.Write(m); err != nil {
					fmt.Fprintf(os.Stderr, "    warning: deactivate write %s: %v\n", id[:min(8, len(id))], err)
				}
				fmt.Fprintf(os.Stderr, "    deactivated memory %s\n", id[:min(8, len(id))])
			}
		}
	}
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

	linkCount := 0
	for _, l := range links {
		link, ok := l.(map[string]any)
		if !ok {
			continue
		}
		// Support both field naming conventions
		idA, _ := link["memory_id_a"].(string)
		if idA == "" {
			idA, _ = link["source"].(string)
		}
		idB, _ := link["memory_id_b"].(string)
		if idB == "" {
			idB, _ = link["target"].(string)
		}
		rel, _ := link["relationship"].(string)
		strength, _ := link["strength"].(float64)

		if _, ok := memByID[idA]; !ok {
			continue
		}
		if _, ok := memByID[idB]; !ok {
			continue
		}

		// Check for duplicate links
		memA := memByID[idA]
		duplicate := false
		for _, existing := range memA.Links {
			if t, _ := existing["target"].(string); t == idB {
				if r, _ := existing["relationship"].(string); r == rel {
					duplicate = true
					break
				}
			}
		}
		if duplicate {
			continue
		}

		memA.Links = append(memA.Links, map[string]any{
			"source":       idA,
			"target":       idB,
			"relationship": rel,
			"strength":     strength,
		})
		if err := s.json.Write(memA); err != nil {
			fmt.Fprintf(os.Stderr, "    warning: link write %s: %v\n", idA[:min(8, len(idA))], err)
		}
		linkCount++
	}

	if linkCount > 0 {
		fmt.Fprintf(os.Stderr, "    → %d new links created\n", linkCount)
	}
	return nil
}

// serializeFull returns full JSON representation of memories for LLM prompts.
func serializeFull(memories []*domain.Memory) string {
	var items []map[string]any
	for _, m := range memories {
		items = append(items, map[string]any{
			"id":             m.ID,
			"summary":        m.Summary,
			"type":           m.Type,
			"confidence":     m.Confidence,
			"importance":     m.Importance,
			"file_paths":     m.FilePaths,
			"tags":           m.Tags,
			"source_commits": m.SourceCommits,
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// serializeCompact returns compact summary for corpus context (less tokens).
func serializeCompact(memories []*domain.Memory) string {
	var items []map[string]any
	for _, m := range memories {
		items = append(items, map[string]any{
			"id":      m.ID,
			"summary": m.Summary,
			"type":    m.Type,
			"tags":    m.Tags,
		})
	}
	data, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(data)
}
