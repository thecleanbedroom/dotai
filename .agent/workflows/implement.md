---
description: "SDLC Step 2/3 — Implement an approved planning document iteratively, working through items one at a time with progress reporting"
---

# /implement — Implement a Planning Document

Execute an approved planning document item by item. Supports resuming across conversation boundaries — reads doc status to pick up where left off.

**Input**: Planning doc path (status must be `Approved` or `In Progress`)
**Output**: Code changes, progress tracked in source doc, blocked items flagged

## SDLC Pipeline

**Full path**: `/plan` → **`/implement`** → `/close`
**Lightweight**: `/capture` (self-contained) | `/hotfix` (fast-track)

**You are here**: `/implement` — executing the approved plan

## Steps

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Resolve input

Follow `/lib`'s _Resolve Input_ step.

### Load the document and plan

Read the resolved source doc and check the frontmatter `> Status:` line:

- **`Approved`**: Fresh start — set status to `In Progress` and begin
- **`In Progress`**: Resuming — scan the `## Progress` section (see _Mark the source document and initialize progress tracking_) to identify what's already done vs remaining
- **`Draft` or `Planned`**: Tell user: "This doc needs planning. Run `/plan`."
- **`Debt`**: Tell user: "This is a debt doc. Run `/plan` to plan the work."
- **`Done`**: Tell user: "This doc is already done. Run `/close` to file it."
- **Anything else**: Tell user: "Unknown status. Check the `> Status:` line."

If an implementation plan artifact exists in the brain directory, load it to understand the reconciliation and work plan. If not, create one using the same structure as `/plan`'s _Create the implementation plan artifact_ step.

// turbo

### Mark the source document and initialize progress tracking

// turbo

- Set the source document's status to `In Progress` (if not already)
- If the source document has a Decisions section, sync decisions from the artifact into it
- **Add a `## Progress` section** to the source document (if it doesn't exist):

```markdown
## Progress

| Phase | Status   | Notes |
| ----- | -------- | ----- |
| 1     | ⬜ Ready |       |
| 2     | ⬜ Ready |       |
```

Phase statuses: `⬜ Ready` | `🔧 In Progress` | `✅ Done` | `🚫 Blocked` | `🗑️ Debt`

This table is the **single source of truth** for resumption.

### Implement iteratively

Work through items from the Progress table. **Before touching any code**, triage for parallelism.

#### Parallelism triage (MANDATORY — do this first)

Scan the Work Plan table for `parallel:X` annotations:

1. **Parallel groups exist**: dispatch subagents FIRST, then work on sequential items while agents run.
2. **No annotations**: ask — "Could any phases run independently?" (different files = candidate). If yes, annotate and dispatch.
3. **Everything sequential**: proceed to sequential items.

> [!IMPORTANT]
> **Dispatch parallel work before starting sequential work.** Pattern: dispatch agents → work on non-conflicting sequential items → check agent results when done.
>
> If you choose NOT to dispatch for `parallel:X` phases, note why in the Progress table.

#### Sequential items (default)

- Pick next `⬜ Ready` or `🔧 In Progress` item
- Set status to `🔧 In Progress` → implement → verify → set `✅ Done`

#### Parallel items

When Work Plan has `parallel:X` annotations:

1. **Group** `⬜ Ready` items sharing the same annotation
2. **Validate file isolation** — no shared files between items. If overlap, fall back to sequential.
3. **Select model** per phase — see `core-parallel-evaluation.md` for tiers
4. **Health check** — `--status` / `--stats` to decide dispatch strategy (`ok` → full batch, `slow` → 1 at a time, `saturated` → do it yourself, `success_rate < 0.7` → sequential)
5. **Dispatch** — use `--batch` (preferred) or single dispatch. See `core-parallel-evaluation.md` for syntax and prompt tips.
6. **Work while agents run** — never idle-poll. If nothing to do, implement the tasks yourself.
7. **Review** — `git diff` each modified file. Verify changes match phase description and acceptance criteria.
8. **Handle outcomes**: acceptable → keep; minor issues → fix inline; wrong approach → revert (`git checkout -- <files>`), re-dispatch (max 2 retries, then sequential).
9. **Verify** — run test suite after entire batch accepted.
10. **On success**: mark `✅ Done`. **On failure**: revert batch, re-implement sequentially.

If any dispatch returns exit code 2 (`QUEUE_FULL`): do that work yourself.

**Between items**, briefly report progress (completed, next, blockers).

> [!IMPORTANT]
> **Update progress after each item** — mark the source doc Progress table `✅ Done`, update the brain task artifact checklist `[x]`, and update the task boundary summary. This is part of the item's work, not a batch operation at the end.
>
> **Sniff while you implement**: apply `/sniff`'s _Smell checklist_ to files you touched and their neighbors. Append any findings to the source doc's `## Debt` section using `/sniff`'s _Logging format_.

### Handle test failures

- **Caused by your changes**: Fix inline as part of the current phase. Do NOT park as debt.
- **Pre-existing**: Fix inline, re-run. All failures must be resolved.
- **Design issues**: Stop and discuss with user before proceeding.

Include test file updates as part of the phase they belong to — no separate "fix tests" phase.

### Handle blockers

When an item cannot be completed (missing dependency, undecided architecture, out of scope, external blocker):

**Do NOT skip silently.** Flag to user, confirm parking. Mark `🚫 Blocked` or `🗑️ Debt` with notes.

### File discovered issues as debt (do NOT fix inline)

Encountered code that looks wrong but is unrelated to your task:

1. Create a debt doc in `docs/` with `> Status: Draft` and datetime-prefixed naming
2. Describe problem, impact, suggested fix. Link to parent doc if applicable.
3. Continue current work — do NOT fix inline.

> [!CAUTION]
> **Never fix discovered issues inline** (scope creep). **Never just mention them in chat** (gets lost). One debt doc per issue, filed immediately.

### Session boundary (if stopping mid-work)

// turbo

- Ensure Progress table is current (✅ completed, 🔧 current with stop-point note)
- Status stays `In Progress`

Next session: `/implement` same doc → resumes from Progress table.

### Run static analysis (mandatory gate)

After all phases complete and tests pass:

// turbo

Discover and run the project's static analysis tool (check Makefile targets, package.json scripts, or project config). If none is configured, skip.

**Hard gate** — do not report completion with errors. Fix errors inline, re-run. All errors must be resolved. Repeat until clean.

### Report completion

Summarize: items completed, items parked as debt, follow-up actions.

Tell the user: `Implementation complete. Review the changes — if anything needs adjusting, describe it here and I'll apply it. When satisfied, run /close docs/<filename>.md to finalize.`

#### Handling review feedback

If the user requests changes after this point:

- Apply the changes as continuation of the current implementation
- Add a `Review` row to the Progress table: `🔧 In Progress` → `✅ Done` with a brief note of what was adjusted
- Re-run tests and static analysis (_Run static analysis (mandatory gate)_) if the changes are non-trivial
- Status stays `In Progress` throughout
