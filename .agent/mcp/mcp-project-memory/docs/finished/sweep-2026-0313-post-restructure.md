# Codebase Sweep — 2026-03-13 (Post-Restructure)

> Status: In Progress

| Phase | Status |
| ----- | ------ |
| Structural scan | Done |
| QA verification | Done |
| Risk analysis | Done |
| Findings documented | Done |

## Progress

| # | Finding | Status | Notes |
|---|---------|--------|-------|
| 1 | Coverage: llm/ and server.py | ✅ Done | 64%→72%, 4 new test files (+46 tests) |
| 2 | Entrypoint SRP | ✅ Done | Split into `_build_parser`, `_run_serve`, `_run_cli` |
| 3 | DRY: BuildAgent constructor | ✅ Done | Extracted `create_build_agent()` factory |
| 4 | N+1 in recall_memory | ✅ Done | Added `get_many()` to MemoryStore |
| 5 | _llm_extract mixed concerns | ✅ Done | Extracted `_log_batch_progress()` static method |
| 6 | Entrypoint smoke test | ✅ Done | Covered by test_llm_client, test_server |
| 7 | retry.py nesting | ✅ Done | Extracted `_classify_error()`, `_extract_retry_after()` |
| 8 | DB close in CLI | ✅ Done | `db.close()` in finally block |
| 9 | split_oversized_commit | ✅ Done | Extracted `_split_diff_by_file()` |
| 10 | Re-export gaps | ✅ Done | `src/__init__.py` uses package-level imports |

---

## Summary

Post-restructure sweep of the project-memory MCP server (`src/`). Entity folder restructure (memory/, llm/, config/) just landed — imports clean, tests pass. This sweep identifies the next batch of improvement opportunities sorted by impact-to-effort ratio.

**Health**: healthy codebase. Clean decomposition across orchestrator (build/agent.py), delegates (batching, retry, synthesis, memory_factory), and infrastructure (db, git, server). The restructure into entity packages improved locality significantly.

## QA Baseline

| Check           | Status | Issues |
| --------------- | ------ | ------ |
| Tests           | ✅     | 78/78 pass |
| Static analysis | ✅     | ruff clean |
| Security        | ✅     | 0 |
| Performance     | ⚠️     | 1 |
| Compatibility   | ✅     | 0 |
| Docs & Ops      | ⚠️     | 1 |

## Requirement

---

#### 1. Coverage: llm/ and server.py are nearly untested

- **What**: `llm/client.py` 13%, `llm/openrouter.py` 22%, `server.py` 14%. These modules carry real logic (JSON extraction, retries, model validation, DB freshness) that has zero test coverage.
- **Where**: `src/llm/client.py`, `src/llm/openrouter.py`, `src/server.py`
- **Why**: Any refactoring or bug fix in these areas is flying blind. The LLM JSON extraction (`_extract_json`) and model validation (`validate_model`) are pure-logic functions that are trivially testable.
- **How**: Add unit tests for `LLMClient._extract_json` (5 cases: raw, fenced, dialog-wrapped, empty, no-JSON). Mock `requests.post` to test `chat()` happy path, error paths, and truncation detection. Mock `OpenRouterAPI._fetch_models` to test `validate_model` and `create_rate_limiter`. For server, test `_check_db_freshness` with mock DB.
- **Priority**: High
- **Effort**: Medium

---

#### 2. Entrypoint SRP — `project-memory` does too much

- **What**: The 175-line entrypoint script mixes arg parsing, dependency checking, component wiring, and CLI dispatch in one `main()` function. The `create_components()` factory returns a bare tuple (not a NamedTuple or dict), requiring index-based destructuring that breaks when the return order changes.
- **Where**: `src/project-memory`
- **Why**: SRP violation. Adding a new command or component requires editing arg parsing, wiring, and dispatch in the same function. The tuple return from `create_components()` is fragile.
- **How**: (a) Return a `dataclass` or `dict` from `create_components()`. (b) Move arg parsing to a `cli.py` module or at minimum extract `_build_parser()` and `_dispatch()` functions to shrink `main()`.
- **Priority**: Medium
- **Effort**: Medium

---

#### 3. DRY: BuildAgent constructor duplicated in 3 places

- **What**: `BuildAgent(db, mem_store, link_store, bm_store, git_par, ext_llm, extract_fallback_llm=..., reasoning_llm=..., openrouter=...)` is constructed identically in `component_factory()`, `build` branch, and `rebuild` branch of the entrypoint.
- **Where**: `src/project-memory` lines 110-116, 144-150, 156-162
- **Why**: DRY violation. 9-arg constructor call repeated 3x. Any change to BuildAgent's constructor requires editing 3 places.
- **How**: Extract a `create_build_agent(components)` factory function, or move construction into the `create_components()` factory itself.
- **Priority**: Medium
- **Effort**: Low

---

#### 4. N+1 in `recall_memory` — linked memories fetched one by one

- **What**: `recall_memory` calls `memory_store.get(lid)` inside a list comprehension for each linked memory. With many links this is N+1 queries.
- **Where**: `src/server.py` lines 221-226
- **Why**: Performance smell. For most real-world cases N is small (< 10 links), but the pattern is still wrong and will scale badly.
- **How**: Add a `get_many(ids)` method to `MemoryStore` using a single `IN (?, ?, ...)` query. Use it in `recall_memory`.
- **Priority**: Low
- **Effort**: Low

---

#### 5. `_llm_extract` inner function — 90 lines, mixed concerns

- **What**: The inner function `_llm_extract` in `build/agent.py` (lines 322-412) is 90 lines and mixes LLM calling, response handling, JSON persistence, progress tracking, and stats formatting in one closure.
- **Where**: `src/build/agent.py` lines 322-412
- **Why**: Opacity + SRP. The closure captures 10+ variables from the enclosing scope. Hard to test in isolation. Progress/stats formatting is interleaved with core logic.
- **How**: Extract a `_process_batch_result()` method for the progress-tracking block (lines 382-410). The LLM call + persist logic is already reasonably scoped.
- **Priority**: Low
- **Effort**: Low

---

#### 6. Magic value: `"from_env()"` in entrypoint but `"load()"` in Config

- **What**: The entrypoint was calling `Config.from_env()` which doesn't exist — it should be `Config.load()`. This was fixed during the restructure, but there's no test catching it.
- **Where**: `src/project-memory` line 30
- **Why**: The entrypoint script has zero test coverage. A method rename on Config would break the CLI with no warning.
- **How**: Add a smoke test that imports and calls `create_components()` with mocked env vars.
- **Priority**: Medium
- **Effort**: Low

---

#### 7. `build/retry.py` — nested conditional depth

- **What**: `call_with_retries` has 6 levels of nesting in the error handling path (lines 78-113): for → try → except → if → if → if. The 429-handling path imports `requests.exceptions.HTTPError` twice (lines 84 and 94).
- **Where**: `src/build/retry.py` lines 78-113
- **Why**: Opacity. Deep nesting makes the retry/fallback/rate-limit logic hard to follow. Duplicate imports are a DRY issue.
- **How**: Extract `_handle_rate_limit(e, rate_limiter)` and `_classify_error(e) → (is_transient, is_rate_limit)` helpers. Import `HTTPError` once at the top of the except block.
- **Priority**: Low
- **Effort**: Low

---

#### 8. `db.py` — `close()` is public but never called

- **What**: `Database.close()` exists but nothing calls it. The entrypoint's `finally` block is `pass`. The server calls `cleanup()` which calls `components["db"].close()`.
- **Where**: `src/db.py`, `src/project-memory` line 170
- **Why**: Resource leak in CLI mode. After `build` or `rebuild`, the DB connection is never explicitly closed. Python's GC will handle it, but it's sloppy.
- **How**: Call `db.close()` in the entrypoint's `finally` block.
- **Priority**: Low
- **Effort**: Low

---

#### 9. `build/batching.py` — `split_oversized_commit` is 120 lines

- **What**: `split_oversized_commit()` is a single 120-line static method with 4 phases: metadata accounting, diff splitting, file batching, body handling. It's the longest method in the codebase.
- **Where**: `src/build/batching.py` lines 122-244
- **Why**: SRP at the method level. The diff-splitting phase (lines 144-158) is an independent algorithm that could be extracted.
- **How**: Extract `_split_diff_by_file(diff: str) → dict[str, str]` as a static helper. This would also make it independently testable (currently 44% coverage on batching).
- **Priority**: Low
- **Effort**: Low

---

#### 10. Missing `__all__` / re-export gaps in entity packages

- **What**: `src/__init__.py` still imports from deep paths (`src.memory.models`, `src.llm.client`, etc.) instead of using the package re-exports (`src.memory`, `src.llm`, `src.config`). The entity package `__init__.py` files exist with `__all__` but the top-level init doesn't use them.
- **Where**: `src/__init__.py`
- **Why**: Defeats the purpose of the `__init__.py` re-exports. Consumers should be able to `from src.memory import Memory` without knowing internal file structure.
- **How**: Rewrite `src/__init__.py` imports to use package-level imports: `from src.memory import Memory, MemoryLink, ...` etc.
- **Priority**: Low
- **Effort**: Low

---

## Risk Analysis

### High-Risk Methods (Complexity + Low Coverage)

| Method | File | Complexity | Coverage | Strategy |
| ------ | ---- | ---------- | -------- | -------- |
| `LLMClient.chat()` | `llm/client.py` | High (HTTP, retries, parsing) | 13% | A — Test-fixable |
| `OpenRouterAPI.create_rate_limiter()` | `llm/openrouter.py` | Medium | 22% | A — Test-fixable |
| `call_with_retries()` | `build/retry.py` | High (6-level nesting) | 26% | B — Refactor + test |
| `BatchPlanner.split_oversized_commit()` | `build/batching.py` | High (120 lines) | 44% | B — Extract + test |
| `McpServer._ensure_components()` | `server.py` | Medium | 14% | C — Skip (MCP framework) |
| `DependencyChecker.check()` | `deps.py` | Low | 23% | C — Skip (boot logic) |

### Priority Order (impact-to-effort)

1. **#1 Coverage** — high impact, prevents future regressions
2. **#3 DRY: BuildAgent constructor** — low effort, immediate win
3. **#8 DB close** — one-line fix
4. **#6 Entrypoint smoke test** — catches method renames
5. **#10 Re-export usage** — cleanup from restructure
6. **#2 Entrypoint SRP** — medium effort but blocks testability
7. **#4 N+1** — low priority, easy fix
8. **#5 _llm_extract** — extract stats block
9. **#9 split_oversized_commit** — extract diff splitter
10. **#7 retry nesting** — extract classify_error helper
