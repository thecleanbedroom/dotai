---
description: "SDLC Shortcut — Capture ad-hoc work, create a doc from what was just done in conversation and file it to finished/"
---

# /capture — Capture Ad-Hoc Work

Lightweight workflow for reactive work: bug fixes, hotfixes, and small follow-ups. Creates a doc from what was just done and files it. Produces the same output as the full SDLC path — a doc in `finished/` with decisions, changes, and verification.

**When to use**: The change is obvious, scope is small (1-5 files), and there's no architectural decision needed. If you find yourself needing reconciliation or cross-file architectural decisions, switch to `/plan` instead.

**Input**: Either a description of a bug to fix, OR nothing (retroactive — document what was just done in this conversation)
**Output**: Doc created, changes applied (if proactive), doc moved to `finished/`

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: **`/capture`** (self-contained) | `/hotfix` (fast-track)

**You are here**: `/capture` — documenting and filing ad-hoc work

## Modes

### Proactive mode

Run `/capture <description>` before starting work. Fix and document simultaneously.

### Retroactive mode

Run `/capture` after ad-hoc work is already done. The agent reconstructs what changed from:

- Conversation context (files edited, commands run, decisions made)
- `git diff` or `git status` if helpful

This is the most common mode — you fixed something quickly and want to capture it.

## Steps

### 1. Create the doc

// turbo

Create a doc using the standard naming convention:

```
docs/YYYY-MM-DDTHHMM--<slug>.md
```

Use this template:

```markdown
# <Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: In Progress

## Problem

What was broken or needed changing. Include error messages, logs, or reproduction steps.

## Changes

### <Change Group Name>

- What was changed and why
- Link to affected files with [filename](file:///path)

## Verification

How the fix was verified — test output, live testing results, manual checks.

## Decisions

1. **<Topic>**: <what was decided> — Rationale: <why>
```

In retroactive mode, populate all sections from the conversation context. In proactive mode, fill Problem first and update the rest as you go.

### 2. Fix (proactive mode only)

// turbo

If the fix hasn't been applied yet, do it now:

- Make the code changes
- Update the doc's Changes section with file links
- Run tests / verify
- Add results to the Verification section
- Record any decisions made

### 3. Append walkthrough and close

// turbo

Append the standard walkthrough section (same format as `/close`'s "Append walkthrough" step):

```markdown
## Walkthrough

> Executed: YYYY-MM-DD HH:MM (local)

### Files Created

| File | Purpose |
| ---- | ------- |

### Files Modified

| File | Change |
| ---- | ------ |

### Decisions Made

1. **<Topic>**: <answer> — Rationale: <why>
```

Then finalize and move to `finished/`:

- Set status to `Done`
- Add `> Finished: YYYY-MM-DD HH:MM (local)`
- Update Changelog if present
- Move to `finished/` with current time prefix:

```bash
mv docs/YYYY-MM-DDTHHMM--<slug>.md docs/finished/YYYY-MM-DDTHHMM--<slug>.md
```

Report to the user what was documented and where it was filed, along with a **copy-paste ready** git commit message:

```
fix(slug): brief description of what was fixed

- Change 1
- Change 2

Closes: docs/<filename>.md
```

Use `fix` for bug fixes, `feat` for features, `refactor` for restructuring. Derive the scope from the doc slug.
