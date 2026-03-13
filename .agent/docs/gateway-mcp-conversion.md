# Gateway MCP Conversion

Convert `gemini-gateway` from CLI-only to an MCP server so Antigravity calls gateway functions as native tools instead of constructing shell commands.

**Motivation**: Agent never uses the CLI despite rules mandating it. MCP tools are first-class — zero friction, structured I/O, automatic discovery.

**Status**: Planned, not started.

---

## Design Decisions

- **Blocking dispatch** — MCP tool runs the gemini subprocess and returns when done. Simplest correct approach. If client-side timeouts are an issue, add async later.
- **CLI untouched** — existing `gemini-gateway` stays as-is. We're wrapping, not replacing.
- **All files inside package** — no loose files at bin root besides the bash wrapper.

---

## Directory Layout

```
.agent/bin/
├── mcp-gemini-gateway                # Bash wrapper (Antigravity MCP config points here)
├── mcp-gemini-gateway/               # Package directory
│   ├── __init__.py
│   ├── config.py                     # CONFIG dict, SCHEMA_SQL
│   ├── db.py                         # GatewayDB class
│   ├── pacing.py                     # PacingManager class
│   ├── dispatch.py                   # dispatch(), resolve_model(), detect_rate_limit()
│   ├── batch.py                      # cmd_batch(), bucket helpers
│   ├── commands.py                   # cmd_status/jobs/pacing/stats/errors/cancel/retry/tail
│   ├── server.py                     # MCP server — FastMCP tool registration
│   ├── cli.py                        # argparse main() for direct CLI use
│   ├── tests/
│   │   └── test_gateway.py
│   └── data/
│       └── gateway.sqlite
├── gemini-gateway                    # UNCHANGED — existing CLI script
└── update
```

---

## MCP Tools

| Tool | Parameters | Returns | Hint |
|---|---|---|---|
| `gateway_dispatch` | `model`, `prompt`, `label?`, `cwd?`, `sandbox?` | `{job_id, status, output, exit_code}` | destructive |
| `gateway_batch` | `jobs[]` | `{batch_id, results[]}` | destructive |
| `gateway_status` | — | Per-model health/capacity | read-only |
| `gateway_jobs` | — | Active jobs list | read-only |
| `gateway_pacing` | — | Per-model pacing state | read-only |
| `gateway_stats` | `last?` | Historical perf stats | read-only |
| `gateway_errors` | `last?` | Recent failed jobs | read-only |
| `gateway_cancel` | `job_id?`, `batch_id?`, `model?` | `{cancelled[], count}` | destructive |

---

## Source Extraction Map

All extracted from the existing 1,545-line `gemini-gateway` with zero logic changes:

| Module | Source lines | Contents |
|---|---|---|
| `config.py` | 33–181 | `CONFIG` dict, `SCHEMA_SQL` constant |
| `db.py` | 228–324 | `GatewayDB` class, `get_script_dir()` |
| `pacing.py` | 457–507 | `PacingManager` class |
| `dispatch.py` | 514–842 | `dispatch()`, `resolve_model()`, `prompt_hash()`, `detect_rate_limit()` |
| `batch.py` | 1281–1540 | `cmd_batch()`, bucket helpers |
| `commands.py` | 844–1280 | All `cmd_*` functions |
| `cli.py` | 350–452 | `build_parser()`, `main()` |

---

## Verification

```bash
# Existing 26 tests pass with new imports
python3 .agent/bin/mcp-gemini-gateway/tests/test_gateway.py

# Original CLI still works
.agent/bin/gemini-gateway --status

# MCP server starts
.agent/bin/mcp-gemini-gateway serve
```
