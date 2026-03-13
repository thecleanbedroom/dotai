# Project Memory System

Persistent, queryable project knowledge derived from git history. Gives AI coding agents context about decisions, patterns, conventions, and debt — so they stop rediscovering what's already known.

## Quick Start

```bash
cd .agent/memory

# One-time setup — creates venv, installs deps, creates .env
./scripts/install

# Set your API key
# Edit .env and paste your OpenRouter key (https://openrouter.ai/keys)

# Build memories from recent commits (test run)
./project-memory build --limit 20

# Verify it worked
./project-memory inspect stats
```

The script auto-activates its local `.venv` — no need to specify the venv python path.

## Requirements

- Python 3.10+
- `OPENROUTER_API_KEY` env var (for build mode)

Everything else is handled by `scripts/install`.

## Usage

```bash
# MCP server over stdio (for AI agent integration)
./project-memory serve

# Incremental build — self-healing, fills gaps, processes new commits
./project-memory build
./project-memory build --limit 20          # limit to N commits
./project-memory build --synthesis         # re-run link synthesis on existing memories
./project-memory build --yes               # skip cost confirmation

# Full reset — wipes everything, reprocesses all history from scratch
./project-memory reset
./project-memory reset --yes               # skip cost confirmation

# Debug/inspect raw data
./project-memory inspect help
./project-memory inspect tables
./project-memory inspect memories
./project-memory inspect memory 1
./project-memory inspect stats
./project-memory inspect schema
./project-memory inspect fts
./project-memory inspect builds
```

### Build vs Reset

Both commands produce the same end state — a complete set of memories covering your full git history:

- **`build`** — Incremental and self-healing. Skips commits already processed, detects missing memory files and re-extracts those commits. Safe to interrupt and resume.
- **`reset`** — Nuclear option. Wipes all memories, processed state, DB, and build artifacts. Re-extracts everything from scratch.

### Self-Healing

`build` automatically reconciles `processed.json` against actual memory JSON files on disk. If a commit was marked as processed but its memories are missing (e.g., deleted or corrupted), the commit is re-queued for extraction. No manual intervention required.

## Two-Pass Architecture

1. **Pass 1 (Extraction)** — Fast model processes commit batches → raw memories with git commit timestamps
2. **Pass 2 (Synthesis)** — Reasoning model compares new vs existing → accept/reject/link

The `--synthesis` flag re-runs pass 2 on existing memories in a safe `links_only` mode — it can only create links between memories, never delete them.

## MCP Tools

When running in `serve` mode, the following tools are available to AI agents:

| Tool | Purpose |
| ---- | ------- |
| `search_file_memory_by_path` | Query memories by file path or directory |
| `search_project_memory_by_topic` | Full-text search across memory summaries |
| `recall_memory` | Get a specific memory by ID with linked memories |
| `project_memory_overview` | Overview of the knowledge store |
| `memory_inspect` | Debug/inspect system internals |

Each memory includes a `created_at` timestamp derived from the git commit date, plus two scoring fields:
- **confidence** (0-100): computed from evidence signals (commits, files, summary length, tags)
- **importance** (0-100): how much the memory would affect future development

## Configuration

| Env Var | Default | Purpose |
| ------- | ------- | ------- |
| `OPENROUTER_API_KEY` | *(required for build)* | API key for LLM calls |
| `MEMORY_BUILD_MODEL` | auto-detected | Model for memory extraction |
| `MEMORY_REASONING_MODEL` | auto-detected | Model for synthesis pass |
| `MEMORY_BUILD_API_URL` | `https://openrouter.ai/api/v1/chat/completions` | API endpoint |

## Data Layout

```
data/
├── memories/          # Canonical store — one JSON per memory (committed)
├── processed.json     # Set of commit hashes already extracted (committed)
├── project_memory.db  # SQLite cache, rebuilt from JSONs on startup (gitignored)
└── build/             # Temp build artifacts (gitignored, cleared each build)
    ├── *.json         # LLM request/response logs
    └── rate_limit.json
```

## Git Integration

**Auto-build on merge** (optional):

```bash
git config core.hooksPath .githooks
```

This enables the `post-merge` hook which runs `build` and commits the updated DB automatically.

**Structured commits** — use the `/commit` workflow to generate git messages with trailers that produce richer memories:

```
Type: feature|fix|refactor|config|debt|docs|test
Rationale: why this approach
Confidence: high|medium|low
```

## Testing

```bash
cd .agent/memory
./scripts/install  # if not already done
.venv/bin/python -m pytest tests/ -v
```

## Architecture

SOLID class design across focused modules:

| Class | Responsibility |
| ----- | -------------- |
| `Database` | SQLite + schema + FTS5 |
| `MemoryStore` | CRUD, file queries, full-text search |
| `LinkStore` | Bidirectional memory relationships |
| `BuildMetaStore` | Build run tracking |
| `GitLogParser` | Parses `git log` output + trailers |
| `LLMClient` | OpenRouter API via `requests` |
| `OpenRouterAPI` | Dynamic model info, rate limits, cost estimation |
| `RateLimiter` | Cross-process sliding-window rate limiter |
| `BuildAgent` | Orchestrator: extract → synthesize → persist |
| `SynthesisAgent` | Pass 2: accept/reject/link memories |
| `MemoryFactory` | LLM output → Memory with confidence scoring |
| `BatchPlanner` | Commit batching within token budgets |
| `PathFilter` | Exclude paths from extraction |
| `Inspector` | Debug/inspect raw data |
| `McpServer` | 5 tools over stdio |
