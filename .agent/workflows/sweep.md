---
description: "Comprehensive codebase sweep — proposes ideal architecture, evaluates code quality, and diffs against reality. Use with /personas for multi-perspective analysis."
---

# /sweep — Codebase Sweep

Two-phase analysis: first propose how a best-in-class project would organize this code (design), then evaluate what exists against quality standards (eval). Not just "find smells" — design better, then diff against reality.

**Input**: Target path, optional `--work` modifier, optional checklist from caller.
**Output**: Findings using `/lib`'s _Canonical Document Format_ item fields.

## Steps

### Resolve input

Follow `/lib`'s _Resolve Input_ step.

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Research

Follow `/lib`'s _Research Deep_ level for the target path. When called by `/personas`, this step is skipped (already done).

### Consult project memory

> [!IMPORTANT]
> Query the project memory MCP **before** analysis to understand what "better" looks like for THIS codebase.

1. **`search_project_memory_by_topic`** — focus areas (error handling, module boundaries, data flow)
2. **`search_file_memory_by_path`** — memories for files in the target path
3. **`project_memory_overview`** — overall memory landscape

Note: established patterns, past decisions and rationale, known debt, component relationships. The proposal should respect established decisions while improving on them.

---

## Phase 1 — Architectural Design

Propose how a best-in-class version of this project would be organized. Prioritize architectural altitude — ask "how should this system be organized" before "is this function clean."

For each dimension below, state a **structural assertion** — a concrete claim about how code should be organized, based on well-maintained open source patterns:

- ✅ "The build package should not import from storage — it should receive a `JSONStore` interface"
- ❌ "Dependencies should be properly managed"

#### Structure & boundaries

1. **Module boundaries**
   - List all packages/modules. For each, state its single responsibility in one sentence.
   - If a package's responsibility can't be stated in one sentence, it likely owns too many concerns — flag it.
   - Grep for a domain term (e.g., "user", "payment") — if it appears in 3+ packages, ownership is fragmented.

2. **Dependency graph**
   - Trace imports: domain/core packages must not import infrastructure (database, HTTP, filesystem).
   - Flag any bidirectional imports between packages (A imports B and B imports A).
   - Verify dependency direction: domain ← application ← infrastructure ← entrypoint. If the project has no explicit layers, note that as a structural gap.

3. **Abstraction layers**
   - Count layers between entrypoint (main/handler) and data store. Flag > 5 for review or < 2 (missing separation).
   - For each layer boundary, verify the inner layer doesn't leak implementation details to the outer (e.g., SQL types in domain structs, HTTP status codes in business logic).

#### Data & flow

4. **Data flows**
   - Trace each core data type from creation to persistence. Verify exactly one package is responsible for mutating it.
   - Flag data that's serialized, deserialized, and re-serialized between layers without transformation (unnecessary round-trips).
   - Check for redundant passes: data read from DB, transformed, then parts discarded — should the query have been narrower?

5. **Error propagation**
   - Pick 3 representative error paths (validation, external service, data not found). Trace each from origin to final handler.
   - Verify the same wrapping pattern is used consistently (wrap with context? return sentinel? log and swallow?).
   - Check that errors cross layer boundaries with appropriate translation (e.g., SQL errors don't leak to API responses).

#### Cross-file integrity

6. **Cross-file invariants** — Maps used for validation (e.g., `allowedColumns`, `validStatuses`), enum/const groups, and switch statements with explicit cases — do all producers/consumers stay in sync? Trace every such set to its producers and flag drift.
7. **Schema coherence** — Does adding a field require changes in N scattered places (DDL, migration, select list, scanner, struct, whitelist)? Documented or enforced?
8. **Config & documentation symmetry**
   - Code → Docs: grep all config/env var reads in source. Every one must appear in documentation.
   - Docs → Code: every config value in README/docs must be consumed somewhere. Flag stale entries.
   - Uniform access: all config should flow through one mechanism. Flag any that bypass it (e.g., raw `os.Getenv` when everything else uses a config struct).
   - Per-key maps that must cover the same key set — do all maps match? Missing key → silent zero-value?

9. **Integration seam contracts**
   - Data crossing module boundaries should use typed structures, not untyped maps/dicts with implicit key assumptions.
   - When one module writes data and another reads it, verify both agree on shape — key names, types, required vs optional fields.
   - External API responses validated before consumption — type assertions, existence checks, not blind property access.
   - Degradation strategy when upstream contracts shift — what happens when a field is missing or renamed?

#### Resilience & lifecycle

10. **Process lifecycle**
    - What in-memory state would be lost on restart, crash, or eviction? Is it persisted, re-derivable, or documented as ephemeral?
    - Long-lived connections re-established after drops?
    - Session recovery handled without user intervention?

11. **State machine integrity**
    - Transitions guarded to valid source states? (e.g., can't cancel an already-finished job)
    - Control flags (pause/cancel/retry) reset between operations? Previous state doesn't bleed into new operations?
    - Concurrent invocations of the same action idempotent?

12. **Partial failure consistency**
    - In batch/loop operations, if iteration N+1 fails, is iteration N's work preserved, rolled back, or left inconsistent?
    - "Mark all complete" after partial batch → permanent data loss for failed items?
    - Progress tracking updated per-iteration or only at end? Latter risks loss on crash.

#### Project completeness

13. **Documentation-code consistency**
    - Config defaults in README match actual defaults in code — diff every documented value.
    - File paths and data layout descriptions match what code actually produces.
    - CLI commands/flags in docs actually work — verify the code supports them.
    - Architecture diagrams and module descriptions reflect current structure, not pre-refactor.

14. **Version & release readiness**
    - Version strings from single source (build flags, version file) — not hardcoded in multiple places.
    - Module/package paths match intended deployment target (registry, GitHub org).
    - CHANGELOG or release notes exist if version claims ≥ 1.0. Pre-release labeled as such.

15. **Project artifact completeness**
    - Referenced setup files exist: if README/Makefile says `cp .env.example .env`, verify `.env.example` exists.
    - Ecosystem-standard files present (lockfile committed, ignore files, license, contributing guide) proportional to intended audience.
    - Build reproducible from clean checkout with only documented prerequisites.

### Gap analysis

Diff each assertion against actual code. For each gap:

- **Assertion**: how it should be
- **Reality**: how it is (file paths, line references)
- **Before/After**: concrete code sketches (not pseudocode)
- **Why**: impact on the driving question
- **Priority**: High | Medium | Low  •  **Effort**: Low | Medium | High

When no gaps are found, note as **verified** with evidence. Use diagrams for complex before/after.

---

## Phase 2 — Code Quality Evaluation

Walk custom code (not framework/vendor). Apply `/sniff`'s _Smell checklist_ systematically.

#### Standard checks

- SOLID violations (SRP, OCP, LSP, ISP, DIP)
- DRY violations and duplication
- Pattern/anti-pattern detection (platform-appropriate idioms)
- Error handling taxonomy and consistency
- Naming, readability, and clarity
- All `/sniff` checklist categories

#### Cross-cutting checks

These require cross-file analysis that `/sniff` (eyes-only) can't do:

- **String constants** — Repeated string literals as status/error/state values across 3+ files without named constants. Check for spelling variants (e.g., "cancelled" vs "canceled").

- **Test fidelity**
  - Mocks implement exactly the required interface — no extra dead methods (misleading), no missing methods (incomplete).
  - Assertions use exact values when knowable, not loose bounds (≥1 can't detect over-matching).
  - Every test has at least one assertion — tests that only prove "didn't crash" create false confidence.
  - Mocks produce same data shape as production. Simpler output → flag untested branches at serialization boundaries.
  - Error-path tests at reasonable ratio to happy-path tests. Catch/fallback blocks without corresponding tests exercising them.

- **Goroutine/async lifecycle** — Every spawned goroutine/task needs: (1) clear termination condition, (2) cancellation respect, (3) no reference leaks that prevent GC. Flag fire-and-forget without timeout or context.

- **Context propagation** — Flag every `context.Background()` / `context.TODO()` outside entrypoints and tests. Each must have a comment explaining why parent context is not propagated.

- **Map iteration order** — `break` on first map iteration or map→ordered collection without sorting. Either sort keys first or document why order is irrelevant.

- **Nil return contracts** — Functions returning `(pointer, error)` where both nil = "not found" — or equivalent patterns in other languages (`null` + no exception, `None` + `None`, `undefined` result). Verify the contract is documented and all callers check both values consistently.

- **Discarded errors** — Every `_ = fn()` on error-returning functions must have a comment (inline or file-level bulk) explaining why the error is safe to ignore.

- **Timing constants** — All hardcoded durations, timeouts, and backoff values should be config (if user-tunable) or named constants with doc comments (if fixed). Flag bare numeric literals in duration expressions.

- **Boundary input tracing** — For each public API parameter (HTTP, CLI, MCP tool, library function) that reaches a DB query, filesystem operation, or external call, trace from entry to final use:
  - Empty/zero: what happens with empty string, zero, nil, or missing?
  - Oversized: extremely long, deeply nested, or numerically huge values?
  - Substring assumptions: code that assumes minimum length (e.g., `id[:8]`) without bounds check?
  - Type coercion: numeric strings, booleans-as-strings, unicode handled at boundary or silently passed?

- **Resource lifecycle**
  - Pairing: every open/acquire/connect has corresponding close/release/disconnect.
  - Conditional cleanup: defer/finally not inside conditional branches where it might not execute.
  - Error path cleanup: if function acquires resource A then fails on B, verify A is still cleaned up.
  - Background work: spawned workers have join/await/cancel mechanism. Check leak potential in error paths.

Record each finding:

- **What**: observation  •  **Where**: path(s)  •  **Why**: impact  •  **How**: actionable fix
- **Priority**: High | Medium | Low  •  **Effort**: Low | Medium | High

---

## Phase 3 — Checklist, Work & Verification

### Execute checklist

If a checklist is provided by caller, work through each item against actual code with a pass/fail/n-a verdict and evidence. When no external checklist, use `/sniff`'s _Smell checklist_.

> [!IMPORTANT]
> Every checklist item needs evidence. "Looks fine" is not a verdict — cite file, line, and pattern.

### Do the work (when `--work` modifier is active)

- Fix actionable findings directly
- Follow skills guidance for clean, idiomatic changes
- Classify non-fixable items using `/lib`'s _Classification_
- Record what changed and what remains

When in doc mode: report findings only.

### QA verification

Follow `/lib`'s _QA Verification_ step.

### Risk analysis

Follow `/lib`'s _Risk Analysis_ step.

---

## Write findings

Order: architectural gaps first (by impact), then code quality findings (by impact-to-effort ratio).

When zero findings, document clean sweep evidence:

| Assertion | Verified by | Evidence |
|-----------|------------|----------|
| Module boundaries clean | Reviewed all imports | No cross-boundary violations |

When standalone, present via `notify_user`. When called by `/personas`, return for collection.

## Scoping

- User-specified scope → limit to that area
- No scope → full custom codebase
- Prioritize 5–10 architectural + 10–20 quality findings. Quality over quantity.
- Risk analysis: top 10 methods above threshold
