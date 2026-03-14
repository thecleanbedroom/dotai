// Package server provides the MCP server adapter — lazy init, DB freshness
// detection, rebuild orchestration, and dual-transport (stdio + HTTP).
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/dotai/mcp-project-memory/internal/domain"
	"github.com/dotai/mcp-project-memory/internal/inspector"
	"github.com/dotai/mcp-project-memory/internal/storage"
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
	DB        *storage.Database
	JSONStore domain.JSONStore
	DataDir   string

	mcp *mcpserver.MCPServer // set by Init()
}

// Init creates the MCPServer instance, registers tools and prompts.
// Must be called before Start or StartStdio.
func (s *McpServer) Init(memStore *storage.MemoryStore) {
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
		memories, _ := memStore.ListAll(true, 20)
		var sb strings.Builder
		sb.WriteString("# Project Memory Briefing\n\n")
		for _, m := range memories {
			sb.WriteString(fmt.Sprintf("## [%s] %s (importance: %d)\n%s\n\n",
				m.Type, m.ID[:8], m.Importance, m.Summary))
		}
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{Role: "user", Content: mcp.TextContent{Type: "text", Text: sb.String()}},
			},
		}, nil
	})
}

// Start begins serving MCP over streamable HTTP.
func (s *McpServer) Start(ctx context.Context, addr string) error {
	httpServer := mcpserver.NewStreamableHTTPServer(s.mcp)

	srv := &http.Server{
		Addr:    addr,
		Handler: httpServer,
	}

	go func() {
		<-ctx.Done()
		fmt.Fprintln(os.Stderr, "shutting down...")
		srv.Shutdown(context.Background())
	}()

	fmt.Fprintf(os.Stderr, "project-memory MCP server listening on %s (http)\n", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// StartStdio begins serving MCP over stdin/stdout.
func (s *McpServer) StartStdio(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "project-memory MCP server (stdio)")
	stdioServer := mcpserver.NewStdioServer(s.mcp)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// EnsureFresh checks if the SQLite DB is stale compared to JSON files.
// If stale, it rebuilds. Returns true if a rebuild occurred.
func (s *McpServer) EnsureFresh() (bool, error) {
	currentFP, err := s.JSONStore.ComputeFingerprint(s.DataDir)
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

	memStore, ok := s.MemReader.(*storage.MemoryStore)
	if !ok {
		return false, fmt.Errorf("MemReader is not a *storage.MemoryStore")
	}
	linkStore, ok := s.Links.(*storage.LinkStore)
	if !ok {
		return false, fmt.Errorf("Links is not a *storage.LinkStore")
	}

	count, err := storage.RebuildDBFromJSON(s.DB, memStore, linkStore, s.JSONStore, s.DataDir, nil)
	if err != nil {
		return false, fmt.Errorf("rebuild: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  rebuilt DB: %d memories loaded\n", count)
	return true, nil
}
