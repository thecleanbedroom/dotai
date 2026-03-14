---
description: "Audit the project memory system — run tests, validate integrity, estimate costs, and report results"
---

# /audit-memory — Memory System Audit

Run a comprehensive health check of the project memory system. Tests all MCP endpoints, validates data integrity, checks FTS sync, and produces a pass/fail report with cost analysis.

**Input**: None — reads live memory data via MCP tools
**Output**: Audit report artifact with test results and recommendations

## Steps

### Verify MCP server is running

Call `project_memory_overview`. If it fails, stop and report that the MCP server is not running.

### Gather baseline stats

Record from `project_memory_overview`:
- Total memories, type distribution, confidence distribution
- Average importance
- Last build info (type, commit count, date)

If total memories is 0, note that a build is needed (`make build-memories` from the Go project).

### Run database health checks

Call `memory_inspect` with query `fts` and verify:
- `fts_rows` equals `memory_rows`
- `in_sync` is `true`

If out of sync, flag as ❌ **CRITICAL**.

Check for WAL/SHM file presence:

// turbo
```bash
ls -la .agent/mcp/mcp-project-memory/data/mcp-project-memory.sqlite* 2>/dev/null
```

If `-shm` or `-wal` files exist and the MCP server is not actively running as a separate process, flag as ⚠️ warning.

### Run FTS search tests

First, fetch a known memory to use as test data:
- Call `search_file_memory_by_path` with a path from `top_files` in the overview
- Extract a distinctive word from the first result's summary to use as the search term

Then execute these searches using `search_project_memory_by_topic`:

| Test | How | Expected |
|---|---|---|
| Positive match | Search for the extracted word | ≥1 result |
| Negative match | Search for `"xyznonexistent"` | 0 results |
| Type filter | Search with `type=` set to a type from the stats distribution | ≥1 result |
| Importance ceiling | Search with `min_importance=99` | 0 results |

For each, record: query, filters, result count, pass/fail.

### Run file path query tests

Use paths from `top_files` in the overview:

| Test | How | Expected |
|---|---|---|
| Exact file | `search_file_memory_by_path` with a file path from `top_files` | ≥1 result |
| Directory prefix | Strip the filename, query the directory with trailing `/` | ≥1 result |
| Nonexistent path | Query `nonexistent/path/foo.md` | 0 results |

### Run memory retrieval tests

Use a memory ID obtained from a previous query result:

1. Call `recall_memory(memory_id=<id>, include_links=true)`. Verify the response contains: `id`, `summary`, `type`, `confidence`, `importance`, `files`, `source_commits`.
2. Call `recall_memory(memory_id="00000000-0000-0000-0000-000000000000")`. Verify it returns an error.
3. If the retrieved memory has links, verify `linked_memories` is populated and each entry has a valid `id`.

### Run link integrity tests

Call `memory_inspect` with query `links`. For each link verify:
- `memory_id_a` and `memory_id_b` are valid UUIDs
- `relationship` is a known type
- `strength` is between 0 and 100

Also report the relationship type distribution and flag if `related_to` exceeds 80% of all links.

### Estimate build cost

Check the data directory for build logs:

// turbo
```bash
ls -la .agent/mcp/mcp-project-memory/data/ 2>/dev/null
```

If build metadata is available via `memory_inspect` with query `builds`, review the last build's commit count and estimate cost based on the configured model's pricing.

### Compile audit report

Write the report to an artifact. Use this structure:

```markdown
# Memory Audit — <date>

## Database Health
| Metric | Value | Status |
|---|---|---|

## Test Results
| Category | Tests | Passed | Failed | Warnings |
|---|---|---|---|---|

## FTS Search Tests
| # | Query | Filters | Expected | Result | Status |
|---|---|---|---|---|---|

## File Path Query Tests
| # | Path | Expected | Result | Status |
|---|---|---|---|---|

## Memory & Link Tests
| # | Test | Result | Status |
|---|---|---|---|

## Build Cost
| Metric | Value |
|---|---|

## Known Limitations
| Issue | Severity | Notes |
|---|---|---|

## Summary
- ✅ Passed: <count>
- ⚠️ Warnings: <count>
- ❌ Failed: <count>
```

### Present results

Present the audit report to the user for review. If any tests failed, highlight the failures prominently and suggest remediation steps.
