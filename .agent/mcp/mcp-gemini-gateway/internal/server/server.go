package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/gateway"
)

// MCPServer wraps the MCP server and gateway.
type MCPServer struct {
	mcp     *server.MCPServer
	gateway *gateway.Gateway
	logger  *slog.Logger
}

// New creates a new MCP server with all gateway tools registered.
func New(gw *gateway.Gateway, logger *slog.Logger) *MCPServer {
	s := &MCPServer{
		mcp: server.NewMCPServer(
			"mcp-gemini-gateway",
			"1.0.0",
			server.WithToolCapabilities(true),
		),
		gateway: gw,
		logger:  logger,
	}

	s.registerTools()
	return s
}

// Start begins serving MCP over streamable HTTP.
func (s *MCPServer) Start(ctx context.Context, addr string) error {
	httpServer := server.NewStreamableHTTPServer(s.mcp)

	srv := &http.Server{
		Addr:    addr,
		Handler: httpServer,
	}

	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down MCP server")
		srv.Shutdown(context.Background())
	}()

	s.logger.Info("starting MCP server (http)", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// StartStdio begins serving MCP over stdin/stdout.
func (s *MCPServer) StartStdio(ctx context.Context) error {
	s.logger.Info("starting MCP server (stdio)")
	stdioServer := server.NewStdioServer(s.mcp)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// toJSON marshals a value to indented JSON string.
func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error())
	}
	return string(b)
}
