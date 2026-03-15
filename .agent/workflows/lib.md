---
description: "Canonical building blocks referenced by all workflows. Not a workflow itself — running /lib does nothing."
---

# /lib — Workflow Building Blocks

This file is a reference library. It is **not callable** — running `/lib` does nothing. Workflows reference specific sections by name: `Follow /lib's _Section Name_ step.`

---

## Canonical Document Format

Every workflow creates and consumes this format. Single source of truth for doc structure.

```markdown
# <Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: Draft

## Requirement

### <Item Title>

- **What**: what needs to change
- **Where**: file(s) or area affected
- **Why**: why it matters — impact, risk, or motivation
- **How**: concrete, actionable suggestion
- **Priority**: High | Medium | Low
- **Effort**: Low | Medium | High

### <Additional items...>
```

**Naming**: `docs/YYYY-MM-DDTHHMM--<slug>.md`

**Status flow**: `Draft` → `Planned` → `Approved` → `In Progress` → `Done`

**`## Requirement` is a list of items.** Each item is an `h3` with structured fields. All fields are optional — include what's known, omit what isn't.

---

## Resolve Input

Universal first step for workflows that operate on docs. Determines the source doc to work against.

// turbo

| Input | Action |
|-------|--------|
| **Existing doc** | Validate frontmatter, use as source doc. If filename doesn't follow `YYYY-MM-DDTHHMM--<slug>.md`, rename using `> Created:` datetime. |
| **Description** | Create new doc using _Canonical Document Format_ with description as `## Requirement` items. |
| **No explicit input** | Scan conversation context (files edited, commands run, topics discussed). Create doc or locate existing in-progress doc in `docs/`. |

Result: **a resolved doc path.** All subsequent steps use this path.

---

## Research

Context-loading step with three depth levels. Loads project context — does NOT produce artifacts or run QA. If QA is needed, the consumer runs _QA Verification_ separately.

### Quick

Fast orientation. Use for workflows that need basic context.

// turbo

1. List directory tree for the target path
2. Read PROJECT.md (if available)
3. Read all `.agent/rules/` files
4. Identify platform, language, and conventions
5. Query project memory (`search_project_memory_by_topic`, `project_memory_overview`) if MCP available — gracefully skip if not

### Standard

Key files and history. Use for planning and review.

// turbo

Everything in _Quick_, plus:

1. Read key files: entry points, config files, interfaces, public API definitions
2. Check knowledge items for relevant context
3. Scan `docs/finished/` for known debt, past decisions, and recurring friction
4. Read conversation history if relevant

### Deep

Every source file. Use for sweeps, audits, and thorough analysis.

> [!CAUTION]
> You MUST `view_file` on **every source file** in the target path. Do not skip files, do not summarize from memory, do not sample. Complete file-by-file reading is mandatory.

Everything in _Standard_, plus:

1. `view_file` on every source file in scope
2. Full knowledge and history review
3. On follow-up iterations: only re-read **modified files** and their neighbors (importers, callers, structurally similar files) — unchanged files remain in context

---

## Tracing

Investigate code paths for specific concerns. Use when planning changes or assessing impact.

### Trace construction sites

Find all instantiation and injection points for modified classes or interfaces.

### Trace internal dependencies

For moved or refactored methods, verify dependencies exist at the destination. Check import graphs.

### Trace affected tests

Search for tests that mock, instantiate, or assert on changed classes or interfaces. Identify tests that need updating.

### Trace entry point

Follow a specific entry point through its full call chain:

1. Identify the starting point (e.g., public API handler, background job, config value)
2. Follow the full call chain through every layer
3. Note issues along the path — these get elevated priority

---

## Persona Lens Protocol

A persona lens provides a structured perspective for analysis. Any workflow can accept a lens to focus its approach without requiring persona-specific code.

A lens has 5 dimensions:

| Dimension | Purpose |
|-----------|---------|
| **Role** | Who you are and what you prioritize — shapes severity weighting |
| **Reading Order** | How to traverse the codebase — determines attention priority |
| **Driving Question** | The question you ask of every file — guides what you notice |
| **Adversarial Challenge** | What to stress-test against — generates edge-case findings |
| **Entry Point** | Where to start tracing — anchors analysis in a concrete call chain |

---

## Persona Definitions

10 personas ordered: map → read → verify → harden → scale → operate → polish → validate.

### Persona 1: Structure & Naming Architect

| Dimension | Value |
|-----------|-------|
| **Role** | Project structure reviewer — prioritize directory layout, file naming, module boundaries, and whether the tree reveals intent at a glance |
| **Reading Order** | Directory tree first (`find` / `list_dir`), then files top-down — evaluate naming patterns before reading content |
| **Driving Question** | "Can someone understand the architecture from `ls -R` alone?" |
| **Adversarial Challenge** | Rename stress test — "If I moved or renamed this file, would the new name be more discoverable?" Look for misleading names, misplaced files, flat directories that should be grouped, naming inconsistencies across sibling packages |
| **Entry Point** | Start at the root directory tree and work inward, comparing each package's directory name against its actual responsibilities |

### Persona 2: Fresh Eyes & Clarity

| Dimension | Value |
|-----------|-------|
| **Role** | New hire (day one) — prioritize unclear naming, missing context, tribal-knowledge dependencies |
| **Reading Order** | Alphabetical by package — how a newcomer browsing the repo would encounter things |
| **Driving Question** | "What would I need to understand to add a new feature here?" |
| **Adversarial Challenge** | Edge case generator — "What if the input is empty, huge, or malformed?" Look for missing validation, panics on unexpected input |
| **Entry Point** | Trace a configuration value from definition to every place it flows |

### Persona 3: Consistency & Long-Term Maintenance

| Dimension | Value |
|-----------|-------|
| **Role** | Maintainer (2 years later) — prioritize fragile coupling, undocumented decisions, code that resists safe modification |
| **Reading Order** | By dependency depth (most-imported → least) — start with the core that everything depends on |
| **Driving Question** | "Where does this codebase violate its own patterns?" |
| **Adversarial Challenge** | Race condition hunter — "What if two requests hit this simultaneously?" Look for shared mutable state, missing synchronization, TOCTOU bugs |
| **Entry Point** | Trace from a test to understand what it actually exercises and what it misses |

### Persona 4: Contracts & Compatibility

| Dimension | Value |
|-----------|-------|
| **Role** | API consumer — prioritize awkward interfaces, leaky abstractions, surprising behavior, breaking-change risk |
| **Reading Order** | Bottom-up (leaf packages → orchestrators) — see the building blocks before the assembly |
| **Driving Question** | "What's the blast radius if I change this interface?" |
| **Adversarial Challenge** | Backwards compatibility breaker — "What if a caller upgrades to this version?" Look for silent behavior changes, missing deprecation |
| **Entry Point** | Trace a specific error from origin through every layer to its final destination |

### Persona 5: Security & Resilience

| Dimension | Value |
|-----------|-------|
| **Role** | Security auditor — prioritize input validation, auth paths, secrets exposure, injection vectors |
| **Reading Order** | Top-down (entry point → dependencies) — follow the request path to find attack surface |
| **Driving Question** | "What breaks if the network is slow or the database is down?" |
| **Adversarial Challenge** | Fault injection — "What if every external call fails?" Look for missing error paths, unhandled nil/null, crash-on-failure |
| **Entry Point** | Trace from a public API handler/endpoint through its full lifecycle |

### Persona 6: Data Integrity Guardian

| Dimension | Value |
|-----------|-------|
| **Role** | Database/state expert — prioritize transactions, idempotency, migration safety, data consistency |
| **Reading Order** | Bottom-up (data layer → consumers) — see the storage layer before the business logic |
| **Driving Question** | "What happens if this operation partially completes?" |
| **Adversarial Challenge** | Crash-during-write — "What if the process dies between step 2 and step 3?" Look for non-atomic multi-step writes, missing rollbacks, orphaned state |
| **Entry Point** | Trace from a state mutation through its full write path |

### Persona 7: Scale & Performance

| Dimension | Value |
|-----------|-------|
| **Role** | Performance engineer — prioritize bottlenecks under 10x load, unbounded allocations, N+1 patterns |
| **Reading Order** | Largest files first — complexity magnets and SRP violation candidates |
| **Driving Question** | "What gets worse as this project grows?" |
| **Adversarial Challenge** | Resource exhaustion — "What if this runs for days without restart?" Look for memory/thread/connection leaks, unbounded caches |
| **Entry Point** | Trace from a background job or scheduled task through its full execution |

### Persona 8: Ops/SRE

| Dimension | Value |
|-----------|-------|
| **Role** | On-call SRE — prioritize observability, structured logging, error surfaces, deployment safety, config discoverability |
| **Reading Order** | By dependency depth (most-imported → least) — start with the core that everything depends on |
| **Driving Question** | "If this breaks in production, how fast can I find and fix it?" |
| **Adversarial Challenge** | Fault injection — "What if the database is slow, the disk is full, or config is missing?" Look for silent failures, swallowed errors, missing log context |
| **Entry Point** | Trace from a log line or error message backwards to its origin |

### Persona 9: Developer Experience (DX) Advocate

| Dimension | Value |
|-----------|-------|
| **Role** | Developer advocate — prioritize error message quality, CLI/API ergonomics, documentation, contributor friction |
| **Reading Order** | Alphabetical by package — how a new contributor would discover things |
| **Driving Question** | "What would frustrate someone using this for the first time?" |
| **Adversarial Challenge** | Edge case generator — "What if the input is empty, huge, or malformed?" Look for unhelpful error messages, missing defaults, undocumented behavior |
| **Entry Point** | Trace from a user-facing error message to its generation point |

### Persona 10: Testing Expert

| Dimension | Value |
|-----------|-------|
| **Role** | Testing expert — prioritize test quality, coverage gaps, brittle tests, missing edge case coverage |
| **Reading Order** | Test files paired with their source — read each test file alongside the production code it covers |
| **Driving Question** | "Do these tests verify behavior, or are they coupled to implementation details that will break on refactor?" |
| **Adversarial Challenge** | Test saboteur — "If I introduced a subtle bug, would any test catch it?" Look for missing assertions, tests that always pass, untested error paths |
| **Entry Point** | Start with the least-tested module and work toward the best-tested |

---

## Perspective Composition

When applying a persona lens, compose a detailed, context-aware perspective preamble. Do not copy the persona table verbatim — **write a fresh, natural-language block** (2-3 paragraphs) that:

1. Adopts the persona's voice and priorities from the 5 dimensions
2. Incorporates the target path, detected language/platform, and codebase structure
3. Weaves in context from previous iterations (findings fixed, patterns observed) — or notes "first iteration" if none
4. Frames the adversarial challenge in terms of the project's actual architecture
5. Specifies the concrete entry point to trace

Each perspective should be unique to this run — not a static template.

---

## Classification

Classify findings into three categories:

- **Actionable**: Can be addressed now without breaking external contracts. Refactoring IS actionable.
- **Debt**: Would break published APIs, wire formats, or schemas, or requires infrastructure the project lacks.
- **Accepted**: Intentional patterns confirmed by surrounding context.

---

## QA Verification

Run the project's verification tooling. Use repo-standard commands (check Makefile, package.json scripts, or framework conventions).

#### Test suite

Run the project's tests. Record pass/fail status and any failures.

#### Static analysis

Run the project's linters and type checkers. Record clean/warning/error status.

#### Security checks

- Verify no secrets in source, logs, or prompts
- Check parameterized queries (no string concatenation in SQL/queries)
- Verify env vars are in both `.env` and `.env.example`
- Scan for unvalidated external input at boundaries

#### Performance checks

- Flag blocking external calls in request paths
- Verify cache keys are scoped and invalidated correctly
- Flag unbounded queries or collections without limits

#### Documentation and ops

- README/PROJECT updated if commands or behavior changed
- Migrations, env vars, and release risks documented
- Observability: structured logging, redacted sensitive fields, actionable error messages

#### Cleanup

Remove temporary files/fixtures. Verify no unintended files via `git status` (read-only — never stage, commit, or reset).

---

## Risk Analysis

Identify the highest-risk code by combining complexity and coverage data.

#### Run coverage

Run the project's test suite with coverage enabled. Parse the coverage report for per-file and per-method statistics.

#### Identify high-risk methods

For each method/function with coverage data:

- **Complexity**: cyclomatic complexity, nesting depth, method length
- **Coverage**: percentage of statements covered
- **Risk score**: `complexity² × (1 - coverage)³ + complexity`

Threshold for "needs attention": risk score ≥ 30 (or project-defined threshold).

#### Categorize by fix strategy

- **Category A — Test-fixable**: Complex but structurally sound. Adding tests reduces risk.
- **Category B — Refactor-needed**: Too complex, needs structural refactoring.
- **Category C — Skip**: Complex due to framework/boot/config requirements.

---

## Context Reordering

When files are already loaded in context (e.g., after _Research Deep_), use a **reordering directive** instead of re-reading files. This costs zero extra tokens.

Include a directive in the perspective preamble or workflow step:

> "Review the code you've already read. Focus first on [specific area], then work outward to [other areas]. Pay particular attention to [concern]."

Only re-read files when their content has **changed** (e.g., after applying fixes in work mode). Unchanged files remain in context.
