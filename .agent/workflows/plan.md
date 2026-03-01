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

## Steps

### 1. Consolidate multi-doc inputs (if applicable)

// turbo

When multiple source docs are provided, **combine them into a single planning doc and delete the originals** before doing anything else:

1. Read all input docs
2. Create a new consolidated doc with its own slug describing the combined scope:

```markdown
# <Consolidated Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: Draft

## Requirement (Original)

<summarize the combined intent across all source docs>
```

3. Delete the original source docs — their content is absorbed into the new one
4. Continue the remaining steps using the consolidated doc as the single input

If only one doc (or a chat description) is provided, skip this step.

### 2. Capture the intent

Read what the user provided (description, existing doc, or debt doc). For existing docs, check frontmatter `> Status:`:

| Status              | Action                                    |
| ------------------- | ----------------------------------------- |
| No status / `Draft` | Continue — needs planning                 |
| `Planned`           | Skip to step 5                            |
| `Approved`          | Redirect → `/implement`                   |
| `In Progress`       | Redirect → `/implement` to resume         |
| `Done`              | Redirect → `/close` if not in `finished/` |

**Do NOT augment the source doc yet** — read for context only. Source doc is written in step 8. For small fixes already done, use `/capture` instead.

Identify: **Goal**, **Scope**, **Constraints**, **Referenced code**. If intent is unclear, batch all questions into one ask.

### 3. Research the codebase (deep)

// turbo

Investigate actual code for each area the plan touches. **Do NOT make assumptions** — verify everything.

- Read relevant files, classes, interfaces; check for existing patterns
- **Trace construction sites**: find all instantiation/injection points for modified classes
- **Trace internal deps**: for moved methods, verify `$this->` dependencies exist on destination
- **Identify affected tests**: search for tests that mock/instantiate/assert on changed classes

### 4. Ensure a source doc exists (stub only)

// turbo

The source doc is the permanent record that lives in `docs/`. At this stage, create only a minimal stub if one doesn't already exist. **Do not add planning sections yet.**

**If a source doc already exists**: Leave its content as-is. Do not modify it. However, if the filename does not follow datetime-prefixed naming (`docs/YYYY-MM-DDTHHMM--<slug>.md`), rename it now using the `> Created:` datetime from its frontmatter.

**If no source doc exists** (user gave a description in chat): Create a stub:

```markdown
# <Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: Draft

## Requirement (Original)

<paste the user's original requirement or description here>
```

Use datetime-prefixed naming: `docs/YYYY-MM-DDTHHMM--<slug>.md`

That's it — just frontmatter and the requirement. Full planning sections come in step 8.

### 5. Create the implementation plan artifact

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

| Phase | Description   | Files Touched        | Parallelism               |
| ----- | ------------- | -------------------- | ------------------------- |
| 1     | <description> | file1.php, file2.php | `parallel:A`              |
| 2     | <description> | file3.php            | `parallel:A`              |
| 3     | <description> | file1.php, file4.php | `sequential (depends: 1)` |

**Parallelism**: `parallel:X` = concurrent within group; `sequential (depends: N)` = wait. Phases sharing files MUST be sequential.

### Subagent Dispatch Plan

For each parallel group, specify dispatch strategy (read by `/implement`). Include the gateway binary path:

**Gateway**: `.agent/bin/gemini-gateway`

| Group | Phases | Model Tier | Timeout | Rationale      |
| ----- | ------ | ---------- | ------- | -------------- |
| A     | 1, 2   | fast       | 45s     | Simple changes |

Tiers: `quick` / `fast` / `think` / `deep`. If all sequential, write: "No subagent dispatch needed."

## Proposed Changes

Group by component. For each file, note [NEW], [MODIFY], or [DELETE]:

### <Component Name>

#### [MODIFY] [filename.php](../laravel/path/to/file)

Brief description + code sketch if helpful.

## Test Impact

List test files that need updating due to mock/assertion changes:

- [ ] [TestFile.php](../laravel/path/to/TestFile.php) — what needs to change

## Verification Plan

How you'll verify each change works.
```

### 6. Present for review

Present the artifact via `notify_user` with `BlockedOnUser: true`. Keep the message brief: decision count, drift/blockers, and prompt for inline comments. Don't present the source doc.

### 7. Iterate until approved

On user feedback: re-research questioned areas, update artifact only, move answered questions to Decisions Made. **Repeat until approved** ("looks good", "approved", `/implement`, etc.).

### 8. Write the source document

// turbo

Once approved, **now** augment the source document with the finalized plan. This is the permanent record.

**Before writing**: If the doc filename does not follow datetime-prefixed naming (`docs/YYYY-MM-DDTHHMM--<slug>.md`), rename it first using the `> Created:` datetime from its frontmatter.

**For existing docs**: Add the planning sections below existing content. Do NOT rewrite or reorganize the original content.

**For stub docs**: Expand the stub with the full planning sections.

**Never overwrite existing doc content.** Add sections below it. Include: Context, Goals, Current State, Proposal (phases with affected files), Reconciliation table, Decisions, Verification Plan, Changelog.

### 9. Mark as approved

// turbo

- Set the source document's status to `Approved`
- Add a changelog entry with the approval datetime

Tell the user: `Plan approved. Run /implement docs/<filename>.md to begin implementation.`
