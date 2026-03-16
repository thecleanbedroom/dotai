---
description: "SDLC Step 1/3 — Plan a feature or change by reconciling requirements against actual code, producing an implementation plan, and iterating until approved"
---

# /plan — Plan a Feature or Change

Research the codebase, reconcile intent against reality, surface questions, and produce an implementation plan artifact for inline iteration.

**Input**: A description, existing doc, or debt doc | **Output**: Approved planning doc + implementation plan artifact

## SDLC Pipeline

**Full path**: **`/plan`** → `/implement` → `/close`
**Lightweight**: `/capture` (self-contained) | `/hotfix` (fast-track)

**You are here**: `/plan` — researching and producing an implementation plan

**Key principle**: The artifact is the working draft; the source doc is the final record. All iteration happens on the artifact. Source doc is written only after approval.

## Canonical Document Format

See /lib:#Canonical Document Format#.

## Resolve Input

See /lib:#Resolve Input#.

## Steps

### Evaluate skills

Follow /lib:#Evaluate Skills#.

### Consolidate multi-doc inputs (if applicable)

// turbo

When multiple source docs are provided, **combine them into a single doc using /lib:#Canonical Document Format# and delete the originals** before doing anything else:

1. Read all input docs
2. Create a new consolidated doc with its own slug describing the combined scope, using /lib:#Canonical Document Format#. Merge all requirement items under `## Requirement`.
3. Delete the original source docs — their content is absorbed into the new one
4. Continue the remaining steps using the consolidated doc as the single input

If only one doc (or a chat description) is provided, skip this step.

### Resolve input

Follow /lib:#Resolve Input# to obtain the source doc path.

### Capture the intent

Read the resolved source doc. Check frontmatter `> Status:`:

| Status              | Action                                            |
| ------------------- | ------------------------------------------------- |
| No status / `Draft` | Continue — needs planning                         |
| `Debt`              | Continue — needs planning                         |
| `Planned`           | Skip to _Create the implementation plan artifact_ |
| `Approved`          | Redirect → `/implement`                           |
| `In Progress`       | Redirect → `/implement` to resume                 |
| `Done`              | Redirect → `/close` if not in `finished/`         |

**Do NOT augment the source doc yet** — read for context only. Source doc is written in _Write the source document_. For small fixes already done, use `/capture` instead.

Identify: **Goal**, **Scope**, **Constraints**, **Referenced code**. If intent is unclear, batch all questions into one ask.

### Research the codebase

// turbo

Follow /lib:#Research#(level=standard) for the areas the plan touches. Then:

- Follow /lib:#Tracing# — run `trace_construction_sites`, `trace_internal_dependencies`, and `trace_affected_tests` for modified classes
- **Sniff while you research**: apply /sniff:#Smell Checklist# to files you read. Log findings to the source doc's `## Debt` section using /sniff:#Logging Format#.

### Create the implementation plan artifact

Create an **implementation_plan** artifact in the brain directory — the primary working document. **Batch all questions into the artifact**, not chat.

> [!IMPORTANT]
> **Mid-implementation detection**: If most items are already done, flag to user — suggest `/implement` to track remaining work instead of re-planning.

**Required non-empty sections**: Reconciliation, Decisions Needed, Decisions Made. A plan with no open questions is suspicious.

```markdown
# <Plan Title> — Implementation Plan

Source: [<filename>](../docs/<source-doc>.md)

## Reconciliation

For each actionable item, report its status against actual code:

| Item | Intent | Code Reality | Status |
| ---- | ------ | ------------ | ------ |
| ...  | ...    | ...          | ...    |

Statuses: `✅ Confirmed` | `⚠️ Needs verification` | `⬜ Needs implementation` | `🚫 Blocked` | `❌ Drift`

## Decisions Needed

Questions that must be answered before implementation. Number each one:

1. **<Topic>**: <question> (affects phases X, Y)

## Decisions Made

Record answers as they come in. Keep the numbering matched:

1. **<Topic>**: <answer> — Rationale: <why>

## Work Plan

Table of remaining items (done items excluded). Include files and parallelism:

| Phase | Description   | Files Touched | Parallelism               |
| ----- | ------------- | ------------- | ------------------------- |
| 1     | <description> | file1, file2  | `parallel:A`              |
| 2     | <description> | file3         | `parallel:A`              |
| 3     | <description> | file1, file4  | `sequential (depends: 1)` |

**Parallelism**: `parallel:X` = concurrent within group; `sequential (depends: N)` = wait. Phases sharing files MUST be sequential.

### Subagent Dispatch Plan

For each parallel group, specify dispatch strategy (read by `/implement`). Include the dispatch tool:

**Dispatch**: `gateway_dispatch` / `gateway_batch_dispatch` MCP tools

| Group | Phases | Model Tier | Rationale      |
| ----- | ------ | ---------- | -------------- |
| A     | 1, 2   | fast       | Simple changes |

Tiers: `quick` / `fast` / `think` / `deep`. If all sequential, write: "No subagent dispatch needed."

## Proposed Changes

Group by component. For each file, note [NEW], [MODIFY], or [DELETE]:

### <Component Name>

#### [MODIFY] [filename](../path/to/file)

Brief description + code sketch if helpful.

## Test Impact

List test files that need updating due to mock/assertion changes:

- [ ] [TestFile](../path/to/TestFile) — what needs to change

## Verification Plan

How you'll verify each change works.
```

### Present for review

Present the artifact via `notify_user` with `BlockedOnUser: true`. Keep the message brief: decision count, drift/blockers, and prompt for inline comments. Don't present the source doc.

### Iterate until approved

On user feedback: re-research questioned areas, update artifact only, move answered questions to Decisions Made. **Repeat until approved** ("looks good", "approved", `/implement`, etc.).

### Write the source document

// turbo

Once approved, **now** augment the source document with the finalized plan. This is the permanent record.

**Before writing**: If the doc filename does not follow datetime-prefixed naming (`docs/YYYY-MM-DDTHHMM--<slug>.md`), rename it first using the `> Created:` datetime from its frontmatter.

**For existing docs**: Add the planning sections below existing content. Do NOT rewrite or reorganize the original content.

**For stub docs**: Expand the stub with the full planning sections.

**Never overwrite existing doc content.** Add sections below it. Include: Context, Goals, Current State, Proposal (phases with affected files), Reconciliation table, Decisions, Verification Plan, Changelog.

### Mark as approved

// turbo

- Set the source document's status to `Approved`
- Add a changelog entry with the approval datetime

Tell the user: `Plan approved. Run /implement docs/<filename>.md to begin implementation.`
