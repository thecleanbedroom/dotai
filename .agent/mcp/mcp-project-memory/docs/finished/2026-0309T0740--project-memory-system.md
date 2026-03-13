# Project Memory System

> Created: 2026-03-09 07:40 (local)
> Status: In Progress

## Progress

| Phase | Status | Notes |
| ----- | ------ | ----- |
| 1     | ✅ Done | Scaffolding |
| 2     | ✅ Done | DB layer |
| 3     | ✅ Done | MCP server |
| 4     | ✅ Done | Build agent |
| 5     | ✅ Done | Integration |
| 6     | ✅ Done | Tests (54/54 pass) |

## Requirement

### MCP Server (serve mode)

- **What**: Python MCP server exposing `memory_query_file`, `memory_search`, `memory_get`, `memory_stats`, `memory_inspect` tools over stdio
- **Where**: `.agent/memory/project_memory.py`
- **Why**: Gives AI coding agents persistent, queryable project knowledge derived from git history
- **How**: Python `mcp` SDK with stdio transport, SQLite backend with FTS5
- **Priority**: High
- **Effort**: Medium

### SQLite Schema & Database Layer

- **What**: Create SQLite DB with `memories`, `memory_links`, `build_meta` tables, FTS5 virtual table, indexes, and sync triggers
- **Where**: `.agent/memory/project_memory.py` (schema creation), `.agent/memory/project_memory.db` (artifact)
- **Why**: Persistent storage for memories with full-text search and link traversal
- **How**: `sqlite3` stdlib with FTS5, JSON arrays for file paths
- **Priority**: High
- **Effort**: Low

### Build Agent (build mode)

- **What**: Incremental build that processes new git commits since last build, feeds to LLM, produces/updates memories
- **Where**: `.agent/memory/project_memory.py`
- **Why**: Automated knowledge extraction from commit history
- **How**: `git log` parsing, trailer extraction, OpenRouter API (OpenAI-compatible), DB updates, decay application
- **Priority**: High
- **Effort**: High

### Full Rebuild (rebuild mode)

- **What**: Drop all data and rebuild from entire git history
- **Where**: `.agent/memory/project_memory.py`
- **Why**: Manual reset button when the DB drifts or needs regeneration
- **How**: Same as build but starts from scratch with full `git log -p --reverse`
- **Priority**: Medium
- **Effort**: Low (shares build infrastructure)

### Inspect (inspect mode)

- **What**: Debug/query command for AI visibility into raw memory data
- **Where**: `.agent/memory/project_memory.py`
- **Why**: AI-driven debugging and improvement of the memory system
- **How**: CLI subcommand + MCP tool for querying raw data, build logs, FTS health
- **Priority**: Medium
- **Effort**: Low

### Git Hook Integration

- **What**: Post-merge hook that runs the build agent and commits the updated DB
- **Where**: `.githooks/post-merge`, `.gitattributes`
- **Why**: Automate memory updates on merge to main
- **How**: Shell script calling `python .agent/memory/project_memory.py build`, then `git add/commit`
- **Priority**: Low
- **Effort**: Low

### Enhanced Commit Workflow

- **What**: `.agent/workflows/commit.md` for generating structured commits with git trailers
- **Where**: `.agent/workflows/commit.md`
- **Why**: Richer input for the build agent → better memories
- **How**: Workflow step that instructs AI to generate trailer-enhanced commit messages
- **Priority**: Low
- **Effort**: Low

## Context

Detailed spec: [memory-system-spec.md](../memory-system-spec.md)

## Decisions

1. **Location**: `.agent/memory/` — agent infrastructure, not project source
2. **LLM**: OpenRouter with OpenAI-compatible API via `requests`, 3 starting models (Haiku, GPT-4o-mini, Gemini Flash), configurable via `MEMORY_BUILD_MODEL` env var
3. **No export mode**: Replaced with `inspect` for AI-driven debugging
4. **Architecture**: SOLID class design in a single file, ready for future extraction
5. **Data sources**: Git commits only — context comes from commit messages and diffs, not docs files
6. **Date limiting**: First build processes all history. `--limit N` flag for testing
7. **Dependencies**: No venv — script self-checks packages on startup

## Changelog

- 2026-03-09 07:58: Approved after 2 review rounds
