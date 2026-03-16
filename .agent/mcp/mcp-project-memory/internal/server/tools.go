package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RegisterTools registers all MCP tools on the given MCP server.
func RegisterTools(srv *mcpserver.MCPServer, s *McpServer) {
	// search_file_memory_by_path — readOnly, idempotent
	srv.AddTool(mcp.NewTool("search_file_memory_by_path",
		mcp.WithDescription("Call before modifying a file. Returns past decisions, known bugs, debt, and patterns associated with the file path. Supports exact file paths and directory prefixes.\n\nResults are historical context — always verify against current code."),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path or directory prefix to search for")),
		mcp.WithNumber("min_importance", mcp.Description("Minimum importance score filter"), mcp.DefaultNumber(0)),
		mcp.WithNumber("limit", mcp.Description("Maximum results to return"), mcp.DefaultNumber(20)),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), handleSearchByFile(s))

	// search_project_memory_by_topic — readOnly, idempotent
	srv.AddTool(mcp.NewTool("search_project_memory_by_topic",
		mcp.WithDescription("Search project memory by topic. Returns decisions, conventions, and patterns not visible in the code.\n\nUse short queries (2-3 terms). Run multiple searches rather than one compound query.\n\nmatch: \"any\" (OR, default) or \"all\" (AND).\n\nResults are historical context — always verify against current code."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query (2-3 terms recommended)")),
		mcp.WithString("type", mcp.Description("Filter by memory type (decision, pattern, convention, context, debt)")),
		mcp.WithString("match", mcp.Description("Match mode: 'any' (OR) or 'all' (AND)"), mcp.DefaultString("any")),
		mcp.WithNumber("min_importance", mcp.Description("Minimum importance score"), mcp.DefaultNumber(0)),
		mcp.WithNumber("limit", mcp.Description("Maximum results"), mcp.DefaultNumber(20)),
		mcp.WithString("since", mcp.Description("ISO date filter (created after)")),
		mcp.WithString("until", mcp.Description("ISO date filter (created before)")),
		mcp.WithString("exclude_tags", mcp.Description("Comma-separated tags to exclude")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), handleSearchByTopic(s))

	// recall_memory — non-readOnly (Touch updates access tracking), idempotent
	srv.AddTool(mcp.NewTool("recall_memory",
		mcp.WithDescription("Retrieve a specific memory by ID with full detail and linked memories. Use to drill into search results and see connections.\n\nResults are historical context — always verify against current code."),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("UUID of the memory to retrieve")),
		mcp.WithBoolean("include_links", mcp.Description("Include linked memories in response"), mcp.DefaultBool(true)),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), handleRecallMemory(s))

	// project_memory_overview — readOnly, idempotent
	srv.AddTool(mcp.NewTool("project_memory_overview",
		mcp.WithDescription("Overview of project memory — total memory count, breakdown by type (decision, pattern, convention, context, debt), breakdown by confidence (high, medium, low), average importance score, top 10 most-referenced files, and last build info."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), handleOverview(s))

	// memory_inspect — readOnly
	srv.AddTool(mcp.NewTool("memory_inspect",
		mcp.WithDescription("Debug/inspect the memory system internals.\n\nCommands: tables, memories, memory <id>, links, builds, stats, schema, fts, help"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Inspect command to execute")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
	), handleInspect(s))
}

// --- Handler factories ---

func handleSearchByFile(s *McpServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.EnsureFresh()

		path := req.GetString("path", "")
		limit := int(req.GetFloat("limit", 20))
		minImp := int(req.GetFloat("min_importance", 0))

		memories, err := s.MemReader.QueryByFile(path, limit, minImp)
		if err != nil {
			return toolError(err)
		}

		return toolJSON(map[string]any{
			"caveat":  "Results are historical context — always verify against current code.",
			"count":   len(memories),
			"results": memoriesToDicts(memories),
		})
	}
}

func handleSearchByTopic(s *McpServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.EnsureFresh()

		query := req.GetString("query", "")
		opts := domain.SearchOpts{
			Type:          req.GetString("type", ""),
			Match:         req.GetString("match", "any"),
			MinImportance: int(req.GetFloat("min_importance", 0)),
			Limit:         int(req.GetFloat("limit", 20)),
			Since:         req.GetString("since", ""),
			Until:         req.GetString("until", ""),
		}

		if tags := req.GetString("exclude_tags", ""); tags != "" {
			for _, t := range splitComma(tags) {
				opts.ExcludeTags = append(opts.ExcludeTags, t)
			}
		}

		memories, err := s.Searcher.Search(query, opts)
		if err != nil {
			return toolError(err)
		}

		return toolJSON(map[string]any{
			"caveat":  "Results are historical context — always verify against current code.",
			"count":   len(memories),
			"results": memoriesToDicts(memories),
		})
	}
}

func handleRecallMemory(s *McpServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.EnsureFresh()

		memID := req.GetString("memory_id", "")
		includeLinks := req.GetBool("include_links", true)

		m, err := s.MemReader.Get(memID)
		if err != nil {
			return toolError(err)
		}
		if m == nil {
			return toolError(fmt.Errorf("memory %s not found", memID))
		}

		if err := s.MemWriter.Touch(memID); err != nil {
			slog.Warn("touch failed", "id", memID, "err", err)
		}

		result := map[string]any{
			"caveat": "Results are historical context — always verify against current code.",
			"memory": m.ToDict(),
		}

		if includeLinks {
			linkedIDs, err := s.Links.GetLinkedIDs(memID)
			if err != nil {
				slog.Warn("recall: get linked IDs", "id", memID, "err", err)
			}
			if len(linkedIDs) > 0 {
				linked, err := s.MemReader.GetMany(linkedIDs)
				if err != nil {
					slog.Warn("recall: get linked memories", "err", err)
				}
				result["linked_memories"] = memoriesToDicts(linked)
			}
		}

		return toolJSON(result)
	}
}

func handleOverview(s *McpServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.EnsureFresh()

		stats, err := s.MemReader.Stats()
		if err != nil {
			return toolError(err)
		}

		lastBuild, err := s.Builds.GetLast()
		if err != nil {
			slog.Warn("overview: get last build", "err", err)
		}
		stats["last_build"] = lastBuild

		return toolJSON(stats)
	}
}

func handleInspect(s *McpServer) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		result := s.Inspector.Inspect(query)
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(result)},
		}, nil
	}
}

// --- Helpers ---

func toolJSON(data map[string]any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error marshaling response: %v", err))},
			IsError: true,
		}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(b))},
	}, nil
}

func toolError(err error) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
		IsError: true,
	}, nil
}

func memoriesToDicts(memories []*domain.Memory) []map[string]any {
	result := make([]map[string]any, len(memories))
	for i, m := range memories {
		result[i] = m.ToDict()
	}
	return result
}

func splitComma(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
