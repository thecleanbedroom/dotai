---
description: "Sweep the codebase for improvement opportunities — naming, structure, duplication, debt, friction — and produce a prioritized doc"
---

# /sweep — Codebase Sweep

Active, deliberate scan of the codebase for issues. Applies `/sniff`'s _Smell checklist_ exhaustively and runs verification tooling (tests, linters, coverage). Produces a prioritized findings doc.

**Input**: Optional scope (module, directory, feature area).
**Output**: Findings doc in `docs/`, actionable as `/plan` input.

## Steps

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Understand the project

// turbo

Read PROJECT.md (if available at the project root) and scan the codebase structure. Identify the platform, custom code areas, and conventions in place. Read all `.agent/rules/` files — these define the project's standards and are the primary benchmark.

### Review knowledge and history

// turbo

Check knowledge items, conversation history, and `docs/finished/` for known debt, past decisions, and recurring friction.

---

### Structural scan

Walk the custom code (not framework/vendor). Apply `/sniff`'s _Smell checklist_ systematically. Expand each finding into the full-detail format (richer than `/sniff`'s terse _Logging format_ table):

```markdown
#### Finding title

- **Where**: relative path(s)
- **Smell**: category from sniff checklist
- **What**: what you observed
- **Why it matters**: impact on readability, maintainability, or correctness
- **Suggested fix**: concrete, actionable suggestion
- **Effort**: low / medium / high
```

---

### QA verification

Run the project's verification tooling to establish a baseline. Use repo-standard commands (check Makefile, package.json scripts, or framework conventions).

#### Run test suite

Run the project's tests. Record pass/fail status and any failures.

#### Run static analysis

Run the project's linters and type checkers. Record clean/warning/error status.

#### Security sweep

Apply `/sniff`'s _Security smells_ checklist exhaustively. Additionally:

- Scan recent changes for security regressions
- Verify anti-forgery tokens on state-changing endpoints
- Check that new env vars are in both `.env` and `.env.example`
- Verify no secrets in source, logs, or prompts

#### Performance sweep

Apply `/sniff`'s _Performance smells_ checklist exhaustively. Additionally:

- Flag blocking external calls in request path
- Verify cache keys are scoped and invalidated correctly
- Note findings that require profiling tools for confirmation

#### Compatibility and accessibility

- Confirm browser/runtime targets and polyfills
- WCAG AA contrast checks
- `prefers-reduced-motion` guards on animations

#### Documentation and ops

- README/PROJECT updated if commands or behavior changed
- Migrations, env vars, and release risks documented
- Observability: structured logging, redacted sensitive fields, actionable error messages

#### Cleanup

Remove temporary files/fixtures (test caches, `*.bak`, debug output, scratch scripts). Verify no unintended files via `git status` (read-only — never stage, commit, or reset).

---

### Risk analysis

Identify the highest-risk code by combining complexity and coverage data.

#### Run coverage

Run the project's test suite with coverage enabled. Parse the coverage report to get per-file and per-method statistics.

> [!IMPORTANT]
> Use whatever coverage format the project generates (Clover XML, Istanbul JSON, lcov, etc.). The exact command depends on the platform — check Makefile targets and test configuration.

#### Identify high-risk methods

For each method/function with coverage data, evaluate:

- **Complexity**: cyclomatic complexity, nesting depth, method length
- **Coverage**: percentage of statements covered by tests
- **Risk score**: high complexity + low coverage = highest risk

**Risk formula concept** (CRAP): `risk = complexity² × (1 - coverage)³ + complexity`

Threshold for "needs attention": risk score ≥ 30 (or project-defined threshold).

#### Categorize by fix strategy

For each high-risk method, read the source and categorize:

- **Category A — Test-fixable**: Complex but structurally sound. Adding tests reduces risk without code changes.
- **Category B — Refactor-needed**: Too complex, needs structural refactoring. Extract method, extract class, or replace conditional with polymorphism.
- **Category C — Skip**: Complex due to framework requirements, CLI commands, boot/config logic. Not worth optimizing.

#### Prioritize by impact

Sort actionable methods (A + B) by:

1. Risk score (highest = most dangerous)
2. Change frequency — higher churn + high risk = top priority
3. Blast radius — methods called from many places are riskier than isolated ones

---

### Write the sweep report

Create the report doc in `docs/` using standard naming:

```markdown
# Codebase Sweep — <date>

> Created: YYYY-MM-DD HH:MM (local)
> Status: Draft

## Summary

Brief overview of findings and overall health impression.

## QA Baseline

| Check           | Status   | Issues |
| --------------- | -------- | ------ |
| Tests           | ✅/❌    | count  |
| Static analysis | ✅/⚠️/❌ | count  |
| Security        | ✅/⚠️/❌ | count  |
| Performance     | ✅/⚠️/❌ | count  |
| Compatibility   | ✅/⚠️/❌ | count  |
| Docs & Ops      | ✅/⚠️/❌ | count  |

## Structural Findings

### [Category: e.g., Naming, SRP, Security]

#### Finding title

- **Where**: relative path(s)
- **Smell**: category
- **What**: observation
- **Why it matters**: impact
- **Suggested fix**: concrete suggestion
- **Effort**: low / medium / high

## Risk Analysis

### High-Risk Methods (CRAP ≥ 30)

| Method | File | Risk | Complexity | Coverage | Strategy |
| ------ | ---- | ---- | ---------- | -------- | -------- |

## Prioritized Recommendations

Top items by impact-to-effort ratio, each actionable as a `/plan` input.
```

### Present for review

Present the report via `notify_user`. User decides which items to plan.

## Scoping

- If the user specifies a scope (module, directory, feature area), limit the sweep to that area.
- If no scope given, sweep the entire custom codebase but keep findings actionable — don't produce a 200-item list.
- Aim for 10–20 structural findings max. Quality over quantity.
- Risk analysis: show all methods above threshold, but prioritize the top 10.
