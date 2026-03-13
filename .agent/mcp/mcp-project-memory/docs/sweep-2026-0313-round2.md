# Codebase Sweep — 2026-03-13 Round 2

| Phase | Status |
| ----- | ------ |
| Structural scan | Done |
| QA verification | Done |
| Risk analysis | Done |
| Findings documented | Done |

---

## Summary

Second sweep after completing all 10 findings from round 1. Codebase in good shape — clean decomposition, entity packages, QA green. This sweep applies 5 active skills (clean-code, uncle-bob-craft, python-pro, python-testing-patterns, mcp-builder) and the /sniff smell checklist.

**Health**: healthy. 25 source files, 3947 LOC. Well-organized into entity packages (`build/`, `memory/`, `llm/`, `config/`). Entrypoint recently refactored. Orchestrator pattern clear.

## QA Baseline

| Check           | Status | Issues |
| --------------- | ------ | ------ |
| Tests           | ✅     | 124/124 pass |
| Static analysis | ✅     | ruff clean |
| Security        | ✅     | 0 |
| Performance     | ✅     | 0 new |
| Compatibility   | ✅     | 0 |
| Docs & Ops      | ⚠️     | 1 |

## Requirement

---

#### 1. Coverage gaps: `openrouter.py` (22%) and `server.py` (30%)

- **What**: `openrouter.py` has 22% coverage — `_fetch_models`, `get_key_info`, `create_rate_limiter`, `estimate_cost` are all untested. `server.py` improved from 14%→30% but MCP tool handlers (search, recall, overview, inspect, briefing) still have no test coverage.
- **Where**: `src/llm/openrouter.py`, `src/server.py`
- **Why**: These modules carry real branching logic (free vs paid models, stale DB detection, rate limit detection). Refactoring them is flying blind.
- **How**: For `openrouter.py`: mock `requests.get` to test `_fetch_models`, `get_key_info`, `get_model_info`, `validate_model`, `create_rate_limiter`, and `estimate_cost`. For `server.py`: test the MCP tool handler logic by calling the inner functions with mock components (doesn't require an actual MCP server).
- **Priority**: High
- **Effort**: Medium

---

#### 2. `_run_build` is 170 lines — single largest method

- **What**: `BuildAgent._run_build` (lines 193-509) is 317 LOC including nested `_llm_extract`. Even with `_log_batch_progress` extracted, it still does: model validation, budget computation, commit discovery, path filtering, batching, cost estimation, user confirmation, thread pool dispatch, memory persistence, synthesis, DB rebuild, and metadata recording.
- **Where**: `src/build/agent.py` lines 193-509
- **Why**: SRP violation at method level. Testing any one concern requires running the entire pipeline. The method has 7+ distinct phases that could be tested independently.
- **How**: Extract phases into focused methods: `_validate_and_compute_budget()`, `_discover_commits()`, `_confirm_cost()`, `_extract_memories()`, `_run_synthesis()`. Keep `_run_build` as a thin orchestrator that calls them in sequence.
- **Priority**: Medium
- **Effort**: Medium

---

#### 3. `print_lock` + mutable lists as thread state — fragile concurrency pattern

- **What**: `_run_build` uses `completed_count = [0]` and `total_memories_extracted = [0]` (mutable lists) for thread-safe counting, plus a manual `print_lock`. This is a well-known anti-pattern — the list-as-counter trick is unclear and easy to misuse.
- **Where**: `src/build/agent.py` lines 348-351, 419-423
- **Why**: Opacity. A reader has to know the "mutable list as counter" Python trick. Thread-shared mutable state without a clear protocol.
- **How**: Replace with `threading.Lock` + a simple counter class or `collections.Counter`, or use `concurrent.futures` result aggregation instead of in-flight mutation. The simplest fix: use a `dataclass` with a lock.
- **Priority**: Low
- **Effort**: Low

---

#### 4. Duplicate `call_with_retries` invocation in `_llm_extract`

- **What**: Lines 369-395 contain two nearly identical `call_with_retries(...)` calls differing only in `llm` (primary vs fallback) and `fallback_llm` (None vs self._extract_fallback). 12 lines duplicated.
- **Where**: `src/build/agent.py` lines 366-395
- **Why**: DRY violation. Adding a new parameter to `call_with_retries` requires editing both call sites.
- **How**: Select `llm` and `fallback` in two lines, then make one `call_with_retries` call. Same for `used_llm` on lines 400-402.
- **Priority**: Low
- **Effort**: Low

---

#### 5. `create_rate_limiter` prints to stderr — side effect in query method

- **What**: `OpenRouterAPI.create_rate_limiter()` (line 191-194) calls `print(...)` to stderr. A method that *creates an object* should not have output side effects. Same issue in `estimate_cost` callers who print.
- **Where**: `src/llm/openrouter.py` lines 191-194
- **Why**: Violates command-query separation. Makes the method untestable without capturing stderr. Callers should decide whether to print.
- **How**: Remove the print from `create_rate_limiter`. Let `_run_build` (the caller) handle all user-facing output.
- **Priority**: Low
- **Effort**: Low

---

#### 6. `_run_build` cost confirmation + progress prints interleaved with logic

- **What**: Lines 225-346 mix business logic (model validation, budget computation, commit discovery, batching) with 15+ `print()` calls for user output. This is the largest single readability friction in the codebase.
- **Where**: `src/build/agent.py` lines 225-346
- **Why**: Newspaper metaphor violation (clean-code skill): high-level orchestration and low-level formatting interleaved. Makes the method hard to follow.
- **How**: Extract a `_print_build_summary()` or use a structured logger. Alternative: extract all user-facing output into the entrypoint wrapper, passing results from `_run_build` for display.
- **Priority**: Low
- **Effort**: Medium

---

#### 7. `SynthesisAgent.run` — 130 lines, mixed concerns

- **What**: `SynthesisAgent.run()` (lines 33-163) handles batching, JSON serialization, token estimation, user output, and LLM dispatch in one method. At 130 lines, it's the 2nd longest method.
- **Where**: `src/build/synthesis.py` lines 33-163
- **Why**: SRP at method level. The batching/sizing logic (lines 48-91) is an independent algorithm that could be extracted — similar to how `_split_diff_by_file` was extracted from `split_oversized_commit`.
- **How**: Extract `_compute_batch_size(llm, new_memories, existing_json)` → `int` and `_build_synthesis_batches(new_memories, batch_size)` → `list[list[Memory]]`.
- **Priority**: Low
- **Effort**: Low

---

#### 8. MCP tool descriptions lack `annotations` (mcp-builder skill)

- **What**: All 5 MCP tools (`search_file_memory_by_path`, `search_project_memory_by_topic`, `recall_memory`, `project_memory_overview`, `memory_inspect`) lack MCP tool annotations (`readOnlyHint`, `destructiveHint`, `openWorldHint`).
- **Where**: `src/server.py` lines 140-248
- **Why**: MCP best practice (per mcp-builder skill). Annotations help clients understand tool safety and caching potential. All these tools are read-only and should declare `readOnlyHint: true`.
- **How**: Add `annotations={"readOnlyHint": True}` to each `@mcp.tool()` decorator. Check mcp-python-sdk docs for exact syntax.
- **Priority**: Low
- **Effort**: Low

---

#### 9. `RateLimiter._state_file` hardcoded to source tree

- **What**: `RateLimiter.__init__` resolves `_state_file` relative to its own source location (`Path(__file__).resolve().parent.parent / "data" / "rate_limit.json"`). This means the rate limit state lives in the *source* tree, not the *project* tree.
- **Where**: `src/llm/rate_limiter.py` lines 31-33
- **Why**: When the memory system is symlinked into multiple projects (which it is), they correctly share rate limits. But the coupling to `__file__` is implicit and fragile — if `data/` moves, it breaks silently.
- **How**: Accept `data_dir: Path | None = None` in the constructor. Default to current behavior if not provided. Let the entrypoint pass `data_dir` explicitly.
- **Priority**: Low
- **Effort**: Low

---

#### 10. Test organization: no test for `synthesis.py`, `memory_factory.py`, `openrouter.py`

- **What**: Three modules with real logic have zero dedicated test files: `synthesis.py` (81% cov from integration), `memory_factory.py` (72%), `openrouter.py` (22%). Their coverage comes from incidental exercise through `test_build.py`, not targeted unit tests.
- **Where**: `tests/` directory — missing `test_synthesis.py`, `test_memory_factory.py`, `test_openrouter.py`
- **Why**: python-testing-patterns skill: "1:1 mapping between production modules and test files." Incidental coverage is fragile — refactoring the integration test may silently drop coverage.
- **How**: Create `test_synthesis.py` (test `apply_results` with various synth_result shapes), `test_memory_factory.py` (test `from_llm_output` confidence scoring edge cases), `test_openrouter.py` (mock HTTP for model info/rate limits/cost).
- **Priority**: Medium
- **Effort**: Medium

---

## Risk Analysis

### High-Risk Methods (Complexity + Low Coverage)

| Method | File | Complexity | Coverage | Strategy |
| ------ | ---- | ---------- | -------- | -------- |
| `_run_build()` | `build/agent.py` | Very High (317 LOC, 7 phases) | 83% | B — Refactor: extract phases |
| `SynthesisAgent.run()` | `build/synthesis.py` | High (130 LOC, batching + dispatch) | 81% | B — Extract batch sizing |
| `OpenRouterAPI._fetch_models()` | `llm/openrouter.py` | Low | 22% | A — Test-fixable |
| `OpenRouterAPI.create_rate_limiter()` | `llm/openrouter.py` | Medium | 22% | A — Test-fixable |
| `RateLimiter.acquire()` | `llm/rate_limiter.py` | High (file locks, loops) | 56% | C — Skip (process coordination) |
| `McpServer._ensure_components()` | `server.py` | Medium | 30% | C — Skip (MCP framework) |
| `DependencyChecker.check()` | `deps.py` | Low | 23% | C — Skip (boot logic) |

### Priority Order (impact-to-effort)

1. **#1 Coverage: openrouter + server** — highest risk addressed by tests
2. **#10 Test organization: synthesis, memory_factory, openrouter** — structural gap
3. **#4 DRY: duplicate call_with_retries** — quick win, less code
4. **#5 Side effect in create_rate_limiter** — one-line fix
5. **#8 MCP annotations** — spec compliance, easy
6. **#3 Mutable list counter pattern** — clarity improvement
7. **#9 RateLimiter state_file** — explicit > implicit
8. **#2 _run_build SRP** — medium effort, large payoff
9. **#7 SynthesisAgent.run** — extract batch sizing
10. **#6 Interleaved prints** — medium effort, readability payoff
