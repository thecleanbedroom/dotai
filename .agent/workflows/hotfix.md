---
description: "SDLC Shortcut — Fast-track a simple fix through plan → implement → close without implementation doc review"
---

# /hotfix — Fast-Track Fix

Plan, implement, and close a simple fix in one pass. No implementation doc review gate — use best judgement. Only ask questions if the answer is genuinely ambiguous.

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: `/capture` (self-contained) | **`/hotfix`** (fast-track)

**You are here**: `/hotfix` — fast-tracking a simple fix

**Use when**: Small, well-scoped fixes (1–3 files, clear intent, no architectural decisions).
**Don't use when**: Multi-component changes, new abstractions, breaking changes, or anything you'd want a second opinion on. Use `/plan` → `/implement` → `/close` instead.

// turbo-all

## Steps

### 1. Understand the intent

Read the user's input. This can be:

- **A description** in conversation ("fix the timeout in SyncProductJob")
- **An existing doc** (`/hotfix @[docs/some-spec.md]`) — read it for requirements
- **A file reference** (`/hotfix @[path/to/file.php]`) — the fix targets this file

Identify:

- What's broken or missing
- Where in the codebase it lives
- What "done" looks like

**Do NOT ask clarifying questions** unless the fix could go in two genuinely different directions with different consequences. Make reasonable assumptions and state them.

### 2. Research

Investigate the relevant code. Find:

- The file(s) to change
- Existing patterns to follow
- Affected tests

Keep this tight — you already know it's a small fix.

### 3. Create source doc stub

Create a minimal planning doc:

```
docs/YYYY-MM-DDTHHMM--<slug>.md
```

```markdown
# <Title>

> Created: YYYY-MM-DD HH:MM (local)
> Status: In Progress

## Requirement

<one-liner describing the fix>
```

### 4. Implement

Make the code changes. Follow existing patterns. Update or add tests as needed.

### 5. Verify

Run the test suite. Fix any failures caused by your changes. Note pre-existing failures but don't block on them.

### 6. Close

Append a `## Walkthrough` to the source doc with:

- Files created/modified (with links)
- Decisions made (if any)
- Test results

Set status to `Done`, add `> Finished:` timestamp. Move to `docs/finished/` with finish-time prefix.

### 7. Report

Summarize to the user:

- What changed
- Test results
- Provide a copy-paste commit message (conventional commits format)
