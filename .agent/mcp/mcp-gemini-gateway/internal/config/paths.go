// Package config — paths.go centralizes ALL filesystem paths for the gateway.
// Every path derives from ProjectRoot.
package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths holds all derived filesystem paths for the gateway.
// Constructed once from a project root, then passed everywhere.
type Paths struct {
	ProjectRoot string // git root or CWD — top of the project
	DataDir     string // {ProjectRoot}/.agent/mcp/mcp-gemini-gateway/data
	DBFile      string // {DataDir}/mcp-gemini-gateway.sqlite
	EnvFile     string // {MCPDir}/.env (next to binary)
	EnvExample  string // {MCPDir}/.env.example
}

// NewPaths derives all paths from a project root.
func NewPaths(projectRoot string) *Paths {
	mcpDir := filepath.Join(projectRoot, ".agent", "mcp", "mcp-gemini-gateway")
	dataDir := filepath.Join(mcpDir, "data")
	return &Paths{
		ProjectRoot: projectRoot,
		DataDir:     dataDir,
		DBFile:      filepath.Join(dataDir, "mcp-gemini-gateway.sqlite"),
		EnvFile:     filepath.Join(mcpDir, ".env"),
		EnvExample:  filepath.Join(mcpDir, ".env.example"),
	}
}

// EnsureDirs creates the data directory if it doesn't exist.
func (p *Paths) EnsureDirs() error {
	return os.MkdirAll(p.DataDir, 0o755)
}

// ResolveProjectRoot determines the project root directory.
// Priority: PROJECT_ROOT env var → git root from CWD → git root from executable dir → CWD.
func ResolveProjectRoot() string {
	// 1. Explicit env var
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		return root
	}

	// 2. Git root from CWD (works when user runs from project dir)
	if root, err := gitRootFrom(""); err == nil {
		return root
	}

	// 3. Git root from executable directory (works when Antigravity launches
	//    the binary without setting CWD — the binary lives inside the project)
	if exe, err := os.Executable(); err == nil {
		if root, err := gitRootFrom(filepath.Dir(exe)); err == nil {
			return root
		}
	}

	// 4. CWD fallback
	cwd, _ := os.Getwd()
	return cwd
}

// gitRootFrom runs git rev-parse --show-toplevel from the given directory.
// If dir is empty, uses CWD.
func gitRootFrom(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
