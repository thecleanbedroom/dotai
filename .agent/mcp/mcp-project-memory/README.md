# MCP Project Memory

Persistent knowledge extracted from git history — decisions, patterns, conventions, and debt surfaced as MCP tools for AI coding assistants.

## Quick Start

```bash
cd .agent/mcp/mcp-project-memory

# Copy env and configure
cp .env.example .env

# Development (hot-reload via Air)
go install github.com/air-verse/air@latest
air
```

The server starts on `http://127.0.0.1:8770/mcp` by default.

## Requirements

- Go 1.25+
- [Air](https://github.com/air-verse/air) (optional, for hot-reload)
- [OpenRouter API key](https://openrouter.ai/) (only for build pipeline)

## Configuration

Configuration uses env vars (`.env`). Priority: **env > default**.

| Env Var | Default | Purpose |
| ------- | ------- | ------- |
| `PORT` | `8770` | HTTP port |
| `OPENROUTER_API_KEY` | — | API key for LLM extraction/synthesis |
| `MEMORY_BUILD_API_URL` | `https://openrouter.ai/api/v1/chat/completions` | LLM API endpoint |
| `MEMORY_EXTRACT_MODEL` | `auto` | Extraction model (auto-selects from free models) |
| `MEMORY_EXTRACT_FALLBACK_MODEL` | `auto` | Fallback extraction model |
| `MEMORY_REASONING_MODEL` | `auto` | Synthesis/reasoning model |
| `MEMORY_BATCH_TOKEN_BUDGET` | `100000` | Max tokens per extraction batch |
| `MEMORY_BATCH_MAX_COMMITS` | `20` | Max commits per batch |
| `MEMORY_COMMIT_LIMIT` | `0` (all) | Max commits to process per build |
| `MEMORY_IGNORE_PATHS` | `.agent/mcp/mcp-project-memory/data/*` | Comma-separated globs to ignore |
| `MEMORY_EXTRACT_CONCURRENCY` | `10` | Max parallel extraction batches |
| `MIN_CONTEXT_LENGTH` | `32000` | Minimum model context length for auto-selection |

```bash
# .env
PORT=8770
OPENROUTER_API_KEY=sk-or-...
```

> [!NOTE]
> Air automatically reads `.env` and passes env vars to the binary. No dotenv library needed at runtime.

## Antigravity MCP Setup

### 1. Start the server

```bash
cd .agent/mcp/mcp-project-memory && air
```

### 2. Add to Antigravity config

Edit `~/.gemini/antigravity/mcp_config.json`:

```json
{
  "mcpServers": {
    "project-memory": {
      "url": "http://127.0.0.1:8770/mcp"
    }
  }
}
```

> [!IMPORTANT]
> The server must be running before Antigravity connects. Start it in a separate terminal or as a background service.

### Full config example

```json
{
  "mcpServers": {
    "laravel-boost": {
      "command": "/path/to/mcp-laravel-boost",
      "args": []
    },
    "project-memory": {
      "url": "http://127.0.0.1:8770/mcp"
    },
    "gemini-gateway": {
      "url": "http://127.0.0.1:8670/mcp"
    }
  }
}
```

## MCP Tools

| Tool | Read-only | Purpose |
| ---- | --------- | ------- |
| `search_file_memory_by_path` | ✓ | Find memories linked to a file path or directory |
| `search_project_memory_by_topic` | ✓ | Full-text search by topic keywords |
| `recall_memory` | ✓ | Retrieve a specific memory by ID with links |
| `project_memory_overview` | ✓ | Summary stats, type breakdown, top files |
| `memory_inspect` | ✓ | Debug internals (tables, schema, FTS, builds) |

## Architecture

```
cmd/mcp-project-memory/main.go    Entry point, DI wiring
internal/
├── config/      Settings from .env + embedded prompts/schemas
├── domain/      Core types (Memory, MemoryLink) + segregated interfaces
├── storage/     SQLite (pure Go) + MemoryStore, LinkStore, BuildMetaStore, JSONStore
├── inspector/   Debug commands via domain interfaces
├── server/      MCP server adapter + 5 tool registrations
├── git/         Git log parser (subprocess)
├── llm/         OpenRouter HTTP client + rate limiter
└── build/       Build pipeline: extract → batch → synthesize → score
```

## Build Pipeline

The build pipeline extracts knowledge from git history using LLM analysis:

```bash
# Extract memories from unprocessed commits
make build-memories

# Extract + run synthesis pass (triage + linking)
make build-synthesis

# Reset database (rebuild from JSON files)
make reset
```

## Make Targets

| Target | Description |
| ------ | ----------- |
| `make dev` | Start dev server with hot-reload (installs Air if needed) |
| `make build` | Build the binary to `./mcp-project-memory` |
| `make run` | Build and run the server |
| `make install` | Install binary to `GOPATH/bin` |
| `make test` | Run all tests |
| `make coverage` | Tests with per-function coverage report |
| `make coverage-html` | Generate HTML coverage report |
| `make vet` | Run `go vet` |
| `make check` | Run vet + test |
| `make build-memories` | Run the extraction pipeline on unprocessed commits |
| `make build-synthesis` | Run extraction + synthesis (triage + linking) |
| `make reset` | Drop and rebuild SQLite from JSON files |
| `make setup` | First-time setup (copy .env, tidy deps, create data/) |
| `make clean` | Remove build artifacts |
| `make install-mcp` | Register in `~/.gemini/antigravity/mcp_config.json` |
| `make help` | List all targets |

## Data Layout

```
data/
├── mcp-project-memory.sqlite  # SQLite index (rebuilt from JSON)
├── memories/                  # Individual memory JSON files (source of truth)
│   ├── <uuid>.json
│   └── ...
└── llm_logs/                  # LLM request/response logs
```

