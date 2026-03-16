# MCP Gemini Gateway

Rate-limited gateway for dispatching prompts to Google Gemini CLI with adaptive pacing, job queuing, batch dispatch, and full observability — exposed as an MCP server over streamable HTTP.

## Quick Start

```bash
cd .agent/mcp/mcp-gemini-gateway
make setup   # copy .env, tidy deps, create data dir
make dev     # start with hot-reload (auto-installs Air)
```

The server starts on `http://127.0.0.1:8670/mcp` by default.

## Requirements

- Go 1.25+
- [Gemini CLI](https://github.com/google/gemini-cli) installed and on PATH

## Management

All tasks are available via `make`:

| Command | Purpose |
| ------- | ------- |
| `make setup` | First-time setup (copy `.env`, tidy deps) |
| `make dev` | Start dev server with Air hot-reload |
| `make build` | Build the binary |
| `make run` | Build + run |
| `make test` | Run all tests |
| `make coverage` | Tests with per-function coverage report |
| `make coverage-html` | Browsable HTML coverage |
| `make vet` | Run `go vet` |
| `make check` | Vet + test in one command |
| `make clean` | Remove build artifacts |

## Configuration

Configuration uses env vars (`.env`) or CLI flags. Priority: **flag > env > default**.

| Env Var | Flag | Default | Purpose |
| ------- | ---- | ------- | ------- |
| `PORT` | `--port` | `8670` | HTTP port |
| `GATEWAY_DB_PATH` | `--db` | `data/gateway.sqlite` | SQLite path |

```bash
# .env
PORT=8670
GATEWAY_DB_PATH=data/gateway.sqlite
```

> [!NOTE]
> Air automatically reads `.env` and passes env vars to the binary. No dotenv library needed.

## Antigravity MCP Setup

### 1. Start the gateway

```bash
cd .agent/mcp/mcp-gemini-gateway && make dev
```

### 2. Add to Antigravity config

Edit `~/.gemini/antigravity/mcp_config.json`:

```json
{
  "mcpServers": {
    "gemini-gateway": {
      "serverURL": "http://127.0.0.1:8670/mcp"
    }
  }
}
```

If using a custom port (e.g. `PORT=9090`), update the URL accordingly:

```json
{
  "mcpServers": {
    "gemini-gateway": {
      "serverURL": "http://127.0.0.1:9090/mcp"
    }
  }
}
```

> [!IMPORTANT]
> The gateway must be running before Antigravity connects. Start it in a separate terminal or as a background service.

### Full config example

```json
{
  "mcpServers": {
    "laravel-boost": {
      "command": "/path/to/mcp-laravel-boost",
      "args": []
    },
    "project-memory": {
      "command": "/path/to/.agent/mcp/mcp-project-memory/mcp-project-memory",
      "args": []
    },
    "gemini-gateway": {
      "serverURL": "http://127.0.0.1:8670/mcp"
    }
  }
}
```

## MCP Tools

| Tool | Read-only | Purpose |
| ---- | --------- | ------- |
| `gateway_dispatch` | ✗ | Send a prompt to a Gemini model |
| `gateway_batch_dispatch` | ✗ | Dispatch multiple prompts in parallel |
| `gateway_status` | ✓ | Queue status per model with health |
| `gateway_jobs` | ✓ | List active jobs with timing |
| `gateway_pacing` | ✓ | Adaptive rate-limit state |
| `gateway_stats` | ✓ | Historical performance stats |
| `gateway_errors` | ✓ | Recent failures with details |
| `gateway_cancel` | ✗ | Cancel jobs by ID, model, or batch |
| `gateway_retry` | ✗ | Retry a failed job |

## Model Aliases

| Alias | Model |
| ----- | ----- |
| `lite` | gemini-2.5-flash-lite |
| `quick` | gemini-2.5-flash-lite |
| `fast` | gemini-2.5-flash |
| `think` | gemini-2.5-pro |
| `deep` | gemini-3.1-pro-preview |

## Architecture

```
cmd/mcp-gemini-gateway/main.go    Entry point, DI wiring
internal/
├── config/     Config struct with embedded defaults
├── domain/     Shared types + ModelRegistry
├── database/   SQLite store (4 segregated interfaces)
├── pacing/     Adaptive rate-limit manager
├── gateway/    Orchestrator, dispatch, batch, commands
└── server/     MCP server + 9 tool registrations
```

## Data Layout

```
data/
├── gateway.sqlite      # Job queue + pacing state
└── gateway-output/     # Saved Gemini CLI outputs
```
