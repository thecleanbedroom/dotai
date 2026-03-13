---
description: "Audit the project memory system — run tests, validate integrity, estimate costs, and report results"
---

# /audit-memory — Memory System Audit

Run a comprehensive health check of the project memory system. Tests all MCP endpoints, validates data integrity, checks FTS sync, and produces a pass/fail report with cost analysis.

**Input**: None — reads live memory data via MCP tools
**Output**: Audit report artifact with test results and recommendations

## Steps

### Verify MCP server is running

Call `memory_stats`. If it fails, stop and report that the MCP server is not running.

### Gather baseline stats

Record from `memory_stats`:
- Total memories, type distribution, confidence distribution
- Average importance
- Last build info (type, commit count, date)

If total memories is 0, note that a build is needed (`./project-memory rebuild`).

### Run database health checks

Call `memory_inspect` with query `fts` and verify:
- `fts_rows` equals `memory_rows`
- `in_sync` is `true`

If out of sync, flag as ❌ **CRITICAL**.

Check for WAL/SHM file presence:

// turbo
```bash
ls -la .agent/mcp/mcp-project-memory/data/project_memory.db* 2>/dev/null
```

If `-shm` or `-wal` files exist and the MCP server is not actively running as a separate process, flag as ⚠️ warning.

### Run FTS search tests

First, fetch a known memory to use as test data:
- Call `memory_query_file` with a path from `top_files` in the stats
- Extract a distinctive word from the first result's summary to use as the search term

Then execute these searches:

| Test | How | Expected |
|---|---|---|
| Positive match | Search for the extracted word | ≥1 result |
| Negative match | Search for `"xyznonexistent"` | 0 results |
| Type filter | Search with `type=` set to a type from the stats distribution | ≥1 result |
| Importance ceiling | Search with `min_importance=0.99` | 0 results |

For each, record: query, filters, result count, pass/fail.

### Run file path query tests

Use paths from `top_files` in the stats:

| Test | How | Expected |
|---|---|---|
| Exact file | Query a file path from `top_files` | ≥1 result |
| Directory prefix | Strip the filename from a `top_files` entry, query the directory with trailing `/` | ≥1 result |
| Nonexistent path | Query `nonexistent/path/foo.md` | 0 results |

### Run memory retrieval tests

Use a memory ID obtained from a previous query result:

1. Call `memory_get(id=<id>, include_links=true)`. Verify the response contains: `id`, `summary`, `type`, `confidence`, `importance`, `files`, `source_commits`.
2. Call `memory_get(id=999999)`. Verify it returns an error.
3. If the retrieved memory has links, verify `linked_memories` is populated and each entry has a valid `id`.

### Run link integrity tests

Call `memory_inspect` with query `links`. For each link verify:
- `memory_id_a` and `memory_id_b` are valid (exist within the total memory count range)
- `relationship` is a known type (check against the types defined in `src/models.py` RELATIONSHIP_TYPES)
- `strength` is between 0.0 and 1.0

Also report the relationship type distribution and flag if `related_to` exceeds 80% of all links.

### Estimate build cost

Find the most recent build response log:

// turbo
```bash
ls -t .agent/mcp/mcp-project-memory/data/build_responses/*.json 2>/dev/null | head -1
```

If a response file exists, parse token usage. The log file structure is `{request, response}` — usage is at `response.usage`. Extract `prompt_tokens` and `completion_tokens`.

Report the `upstream_inference_cost` from `response.usage.cost_details` if available. If not, estimate using the model's current pricing from OpenRouter.

Also estimate costs for alternative models listed in `.env` comments.

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
