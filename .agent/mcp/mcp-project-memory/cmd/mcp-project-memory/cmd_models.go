package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/config"
	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/llm"
)

func runModels(cfg *config.Settings) {
	// --- Extraction (Pass 1) context budget ---
	// model_context >= batch_token_budget + overhead + output
	//                  100K (default)    + 8K       + 16K output
	batchBudget := cfg.BatchingTokenBudget()
	extractOverhead := config.ExtractionOverheadTokens()
	extractOutput := 16384 // observed extraction output size
	extractMin := batchBudget + extractOverhead + extractOutput

	// --- Synthesis (Pass 2) context budget ---
	// Context is split by ratios from internal.go:
	//   SynthesisContextFillRatio (0.50) → 50% for input corpus
	//   SynthesisThinkingRatio   (0.15) → 15% for reasoning tokens
	//   Remaining                (0.35) → 35% for output JSON
	//
	// These ratios describe how we PLAN to fill the context, not hard limits
	// on max_completion_tokens. The actual completion budget depends on observed
	// usage, not the model's full context:
	//   Observed: ~20K thinking + ~29K output = ~49K total completion
	//   With growth margin: 65K min completion tokens
	//
	// min context: must fit the entire corpus at 50% fill ratio
	fillRatio := config.SynthesisContextFillRatio()
	thinkRatio := config.SynthesisThinkingRatio()
	outputRatio := 1.0 - fillRatio - thinkRatio
	synthesisMin := 262000 // min context to fit observed ~97K corpus at 50% fill
	synthesisMinOutput := 65536 // observed ~49K completion + growth margin

	fmt.Fprintf(os.Stderr, "querying OpenRouter for suitable models...\n")
	fmt.Fprintf(os.Stderr, "  extraction budget:  %dk context (%dk batch + %dk overhead + %dk output)\n",
		extractMin/1000, batchBudget/1000, extractOverhead/1000, extractOutput/1000)
	fmt.Fprintf(os.Stderr, "  synthesis budget:   %dk context (%.0f%% input, %.0f%% thinking, %.0f%% output)\n",
		synthesisMin/1000, fillRatio*100, thinkRatio*100, outputRatio*100)
	fmt.Fprintf(os.Stderr, "  synthesis min out:  %dk (observed ~49k + margin; covers reasoning + output)\n\n",
		synthesisMinOutput/1000)

	router := llm.NewOpenRouter(cfg.APIURL(), cfg.APIKey())
	models, err := router.ListSuitableModels(extractMin)
	if err != nil {
		log.Fatalf("list models: %v", err)
	}

	if len(models) == 0 {
		fmt.Fprintf(os.Stderr, "no suitable models found\n")
		return
	}

	// Partition models into synthesis-suitable and extraction-only
	var synthModels, extractModels []domain.ModelInfo
	for _, m := range models {
		if m.ContextLength >= synthesisMin && m.MaxCompletionTokens >= synthesisMinOutput {
			synthModels = append(synthModels, m)
		} else {
			extractModels = append(extractModels, m)
		}
	}

	printHeader := func() {
		fmt.Fprintf(os.Stderr, "%-55s %8s %8s  %-5s %s\n",
			"MODEL ID", "CONTEXT", "MAX OUT", "TIER", "EST COST/BATCH")
		fmt.Fprintf(os.Stderr, "%-55s %8s %8s  %-5s %s\n",
			strings.Repeat("-", 55), strings.Repeat("-", 8), strings.Repeat("-", 8),
			strings.Repeat("-", 5), strings.Repeat("-", 14))
	}

	printRow := func(m domain.ModelInfo) {
		ctxStr := fmt.Sprintf("%dk", m.ContextLength/1000)
		outStr := fmt.Sprintf("%dk", m.MaxCompletionTokens/1000)
		tier := "paid"
		if m.IsFree {
			tier = "free"
		}
		costStr := "free"
		if !m.IsFree {
			batchCost := m.Pricing["prompt"]*100_000 + m.Pricing["completion"]*16_000
			costStr = fmt.Sprintf("$%.4f", batchCost)
		}
		fmt.Fprintf(os.Stderr, "%-55s %8s %8s  %-5s %s\n",
			m.ID, ctxStr, outStr, tier, costStr)
	}

	fmt.Fprintf(os.Stderr, "=== Extraction + Synthesis models (%d) ===\n", len(synthModels))
	printHeader()
	for _, m := range synthModels {
		printRow(m)
	}

	fmt.Fprintf(os.Stderr, "\n=== Extraction only models (%d) ===\n", len(extractModels))
	printHeader()
	for _, m := range extractModels {
		printRow(m)
	}

	// Show auto-selected recommendations
	fmt.Fprintf(os.Stderr, "\n--- Auto-Selection ---\n")
	if ext, err := router.AutoSelectExtractionModel(extractMin); err == nil {
		tier := "free"
		if !ext.IsFree {
			tier = "paid"
		}
		fmt.Fprintf(os.Stderr, "  extraction: %s (%s, %dk ctx)\n", ext.ID, tier, ext.ContextLength/1000)
	} else {
		fmt.Fprintf(os.Stderr, "  extraction: %v\n", err)
	}
	if reas, err := router.AutoSelectReasoningModel(synthesisMin, synthesisMinOutput); err == nil {
		tier := "free"
		if !reas.IsFree {
			tier = "paid"
		}
		fmt.Fprintf(os.Stderr, "  reasoning:  %s (%s, %dk ctx, %dk max out)\n", reas.ID, tier, reas.ContextLength/1000, reas.MaxCompletionTokens/1000)
	} else {
		fmt.Fprintf(os.Stderr, "  reasoning:  %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "\nOverride with MEMORY_EXTRACT_MODEL and MEMORY_REASONING_MODEL in .env.\n")
}
