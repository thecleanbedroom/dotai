// Package server provides the MCP server adapter — lazy init, DB freshness
// detection, rebuild orchestration, and stdio transport.
package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/inspector"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// McpServer holds all dependencies needed to serve MCP tool requests.
// Depends on domain interfaces — concrete types injected by cmd/main.go.
type McpServer struct {
	MemReader domain.MemoryReader
	MemWriter domain.MemoryWriter
	Searcher  domain.Searcher
	Links     domain.LinkStore
	Builds    domain.BuildMetaStore
	Inspector *inspector.Inspector
	DB        domain.DatabaseManager
	JSONStore domain.JSONStore
	Rebuilder domain.Rebuilder

	freshMu sync.Mutex             // serializes EnsureFresh to prevent parallel rebuilds
	mcp     *mcpserver.MCPServer   // set by Init()
}

// Init creates the MCPServer instance, registers tools and prompts.
// Must be called before Start or StartStdio.
func (s *McpServer) Init(listFn func(bool, int) ([]*domain.Memory, error)) {
	s.mcp = mcpserver.NewMCPServer(
		"project-memory",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithPromptCapabilities(true),
	)

	RegisterTools(s.mcp, s)

	s.mcp.AddPrompt(mcp.Prompt{
		Name:        "briefing",
		Description: "Top 20 memories as a formatted project context summary",
	}, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		s.EnsureFresh()
		memories, err := listFn(true, 20)
		if err != nil {
			fmt.Fprintf(os.Stderr, "briefing: list memories: %v\n", err)
		}
		var sb strings.Builder
		sb.WriteString("# Project Memory Briefing\n\n")
		for _, m := range memories {
			idShort := m.ID
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			sb.WriteString(fmt.Sprintf("## [%s] %s (importance: %d)\n%s\n\n",
				m.Type, idShort, m.Importance, m.Summary))
		}
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{Role: "user", Content: mcp.TextContent{Type: "text", Text: sb.String()}},
			},
		}, nil
	})
}

// StartStdio begins serving MCP over stdin/stdout.
func (s *McpServer) StartStdio(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "project-memory MCP server (stdio)")
	stdioServer := mcpserver.NewStdioServer(s.mcp)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// EnsureFresh checks if the SQLite DB is stale compared to JSON files.
// If stale, it rebuilds. Returns true if a rebuild occurred.
// Mutex-protected: concurrent MCP tool calls serialize here.
func (s *McpServer) EnsureFresh() (bool, error) {
	s.freshMu.Lock()
	defer s.freshMu.Unlock()

	currentFP, err := s.JSONStore.ComputeFingerprint()
	if err != nil {
		return false, fmt.Errorf("compute fingerprint: %w", err)
	}
	if currentFP == "" {
		return false, nil // no data yet
	}

	storedFP, err := s.DB.GetFingerprint()
	if err != nil {
		// DB might be empty/corrupt — rebuild
		return s.rebuild()
	}

	if storedFP == currentFP {
		return false, nil // up to date
	}

	return s.rebuild()
}

func (s *McpServer) rebuild() (bool, error) {
	fmt.Fprintln(os.Stderr, "  rebuilding DB from JSON...")

	count, err := s.Rebuilder.RebuildFromJSON(nil)
	if err != nil {
		return false, fmt.Errorf("rebuild: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  rebuilt DB: %d memories loaded\n", count)
	return true, nil
}
