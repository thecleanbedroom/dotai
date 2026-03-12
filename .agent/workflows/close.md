---
description: "SDLC Step 3/3 — Close a completed planning document by appending a walkthrough, creating a debt doc if needed, and moving to finished/"
---

# /close — Close a Planning Document

Finalize a completed planning document: append walkthrough, create debt doc for parked items, and move to `finished/` with a finish-time filename.

**Input**: Planning doc path, description, or conversation context (via _Resolve Input_)
**Output**: Walkthrough appended, debt doc if needed, source doc moved to `finished/`

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → **`/close`**
**Lightweight**: `/capture` (self-contained) | `/hotfix` (fast-track)

**You are here**: `/close` — finalizing and filing the completed work

## Steps

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Resolve input

Follow `/plan`'s _Resolve Input_ step.

### Load and verify

Read the resolved source doc. Check the frontmatter `> Status:` line:

- **`In Progress`**: Proceed with closing
- **`Done` but not in `finished/`**: Proceed — just needs filing (skip to _Move to finished_)
- **`Draft` or `Planned`**: Tell user: "This doc needs planning. Run `/plan`."
- **`Approved`**: Tell user: "This doc hasn't been implemented yet. Run `/implement`."
- **`Debt`**: Tell user: "This is a debt doc. Run `/plan` to plan the work."
- **Anything else**: Tell user: "Unknown status. Check the `> Status:` line."

#### Verify Progress table

If the document has a `## Progress` table, check that **every item** has a terminal status (`✅ Done`, `🚫 Blocked`, or `🗑️ Debt`).

If any items are `⬜ Ready` or `🔧 In Progress`, list them and ask the user to resolve each: **✅ Done** | **🗑️ Debt** | **🚫 Blocked**. Update table, then proceed.

If no Progress table exists (older docs), infer completion and confirm with user.

### Run tests (mandatory gate)

**Hard gate** — do not proceed until tests are green.

> [!TIP]
> **Skip if already green**: If the full test suite passed earlier in this conversation **and no code changes since**, skip re-running. Note prior result in walkthrough.

#### Discover how to run tests

// turbo

Check in order: Makefile targets → Docker/Lando commands → package.json scripts → framework-specific commands.

> [!IMPORTANT]
> Runtime version mismatch? Find the containerized command — do not skip tests.

#### Run the tests

// turbo

Run full test suite and capture output.

#### Handle failures

- **Your changes**: Fix inline, re-run.
- **Pre-existing**: Fix inline, re-run. All failures must be resolved.
- **Unsure**: Check `git diff` for modified dependencies.

Repeat until green — zero failures required.

### Run static analysis (mandatory gate)

**Hard gate** — do not proceed until clean.

> [!TIP]
> **Skip if already green**: If static analysis passed during `/implement`'s _Run static analysis (mandatory gate)_ step **and no code changes since**, skip. Note in walkthrough.

// turbo

Discover and run the project's static analysis tool (check Makefile targets, package.json scripts, or project config). If none is configured, skip.

- **Your changes**: Fix inline, re-run.
- **Pre-existing**: Fix inline, re-run. All errors must be resolved.

Repeat until zero errors.

### Test new code

Check whether new files/methods created during implementation lack test coverage. **Quality gate** — new code should not be closed without tests.

1. Identify new files from Progress table / walkthrough
2. Check for existing test files matching new classes
3. Write focused tests (happy path, edge cases, boundaries) for uncovered new code
4. Re-run test suite after adding tests

> [!TIP]
> **Skip if already covered**: If tests were written during implementation, verify they exist and move on.

> [!NOTE]
> New code only — extending coverage for pre-existing code is out of scope. File as debt if needed.

### Code smell sweep

Review the `## Debt` section accumulated during `/plan` and `/implement` (via `/sniff`'s _Smell checklist_). Quick-scan any touched files not yet covered. For each new finding, append to the debt table.

> [!IMPORTANT]
> **Do NOT fix smells inline during close** — that's scope creep. File as debt, announce to user, move on.
> **Recursion guard**: When running `/sniff` from `/close`, do NOT invoke `/sniff`'s _Standalone mode_ (which calls back to _Create debt doc_). Append findings to the existing debt table only.

If no debt section exists (older docs), run a focused `/sniff` on the files listed in the Progress table.

### Create debt doc

Canonical step for creating debt documents. Referenced by other workflows that discover debt.

// turbo

Create a doc in `docs/` using datetime-prefixed naming: `docs/YYYY-MM-DDTHHMM--<slug>.md`

When called from `/close`, use the parent doc's slug + `-debt` suffix.

```markdown
# <Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: Debt
> Source: <where this debt was discovered — e.g., /close of <parent doc>, /sniff scan, /testcoverage triage>

## Requirement

### <Item Name>

- **What**: <what needs to change>
- **Why**: <why it's debt — smell category, risk, or reason it was parked>
- **Needed**: <what must happen to resolve>
- **Priority**: High | Medium | Low
- **Effort**: Low | Medium | High

### <Additional items...>

## Evidence

<Optional: supporting data — sniff findings table, coverage stats, code excerpts, CRAP scores>
```

**Key rules:**

- Status is always `Debt` — this signals `/plan` and `/hotfix` that it's ready for action
- Each item in `## Requirement` must be self-contained — enough context to plan without re-researching
- `## Evidence` is optional supporting data, not a substitute for clear requirements
- Debt docs stay in active `docs/` — ready for `/plan` or `/hotfix`

### Append walkthrough to the source document

// turbo

Append a `## Walkthrough` section — the permanent record of what happened:

```markdown
## Walkthrough

> Executed: YYYY-MM-DD HH:MM (local)

### Plan vs Reality

| Phase | Planned     | Outcome | Notes |
| ----- | ----------- | ------- | ----- |
| 1     | Description | ✅ Done | Notes |

### Files Created / Modified

| File                | Purpose/Change |
| ------------------- | -------------- |
| [filename](../path) | Description    |

### Decisions Made

1. **Topic**: answer — Rationale

### Open Debt

Items + link to debt doc (if created).
```

### Finalize the original document

// turbo

- Set status to `Done`
- Add `> Finished: YYYY-MM-DD HH:MM (local)` line
- If debt doc created, add `> Debt: <link>` line

### Move to finished

// turbo

Rename datetime prefix to **current finish time**, move to `finished/`:

```bash
mv docs/2026-02-12T0744--slug.md docs/finished/2026-02-12T1348--slug.md
```

Filename reflects completion time. For module docs, use module's `docs/finished/`.

#### Rebase relative links

// turbo

Moving adds one directory level — prefix each relative path with `../`:

```bash
sed -i 's|(\.\.\/|(../../|g; s|(\.\./|(../../|g' docs/finished/<filename>.md
```

Spot-check links. Update any docs referencing the moved file.

### Report

Summarize: items completed, items as debt, decisions, file paths, test results, follow-ups.

#### Git commit message

Follow `/commit`'s _Generate the commit message_ step.
