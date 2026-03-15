---
description: "SDLC Shortcut — Capture ad-hoc work, create a doc from what was just done in conversation and file it to finished/"
---

# /capture — Capture Ad-Hoc Work

Lightweight workflow for reactive work: bug fixes, hotfixes, and small follow-ups. Creates a doc from what was just done and files it. Produces the same output as the full SDLC path — a doc in `finished/` with decisions, changes, and verification.

**When to use**: The change is obvious, scope is small (1-5 files), and there's no architectural decision needed. If you find yourself needing reconciliation or cross-file architectural decisions, switch to `/plan` instead.

**Input**: A description, existing doc, or conversation context (retroactive)
**Output**: Doc created, changes applied (if proactive), doc moved to `finished/`

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: **`/capture`** (self-contained) | `/hotfix` (fast-track)

**You are here**: `/capture` — documenting and filing ad-hoc work

## Steps

### Evaluate skills

// turbo

Follow `/skills`'s _Evaluate skills_ step.

### Resolve input

Follow `/lib`'s _Resolve Input_ step with this override:

- **Initial status**: `In Progress` (capture documents work that is already done or about to be done)

### Populate the doc

// turbo

Using `/lib`'s _Canonical Document Format_, populate the resolved doc with:

- **`## Requirement` items**: what was broken or needed changing — each item follows the standard fields (What, Where, Why, How, Priority, Effort — include what's known)
- Apply `/sniff`'s _Smell checklist_ to touched files; log findings to a `## Debt` section using `/sniff`'s _Logging format_

In retroactive mode, populate from conversation context (files edited, commands run, decisions made). In proactive mode, fill `## Requirement` first and update as you go.

### Fix (proactive mode only)

// turbo

If the fix hasn't been applied yet, do it now:

- Make the code changes
- Update the doc's `## Requirement` items with file links
- Run tests / verify
- Record any decisions made

### Close

// turbo

- **Walkthrough**: Follow `/close`'s _Append walkthrough to the source document_ step
- **Finalize**: Follow `/close`'s _Finalize the original document_ step
- **Move + rebase**: Follow `/close`'s _Move to finished_ step
- **Report**: Follow `/close`'s _Report_ step — include a copy-paste commit message
