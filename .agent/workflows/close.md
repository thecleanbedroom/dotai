---
description: "SDLC Step 3/3 — Close a completed planning document by appending a walkthrough, creating a debt doc if needed, and moving to finished/"
---

# /close — Close a Planning Document

Finalize a completed planning document: append walkthrough, create debt doc for parked items, and move to `finished/` with a finish-time filename.

**Input**: Planning doc path (status must be `In Progress`)
**Output**: Walkthrough appended, debt doc if needed, source doc moved to `finished/`

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → **`/close`**
**Lightweight**: `/capture` (self-contained) | `/hotfix` (fast-track)

**You are here**: `/close` — finalizing and filing the completed work

## Steps

### 1. Load and verify

Read the target document. Check the frontmatter `> Status:` line:

- **`In Progress`**: Proceed with closing
- **`Done` but not in `finished/`**: Proceed — just needs filing (skip to step 5)
- **`Draft` or `Planned`**: Tell user: "This doc needs planning. Run `/plan`."
- **`Approved`**: Tell user: "This doc hasn't been implemented yet. Run `/implement`."
- **Anything else**: Tell user which step to run based on the status.

#### Verify Progress table

If the document has a `## Progress` table, check that **every item** has a terminal status (`✅ Done`, `🚫 Blocked`, or `🗑️ Debt`).

If any items are `⬜ Ready` or `🔧 In Progress`, list them and ask the user to resolve each: **✅ Done** | **🗑️ Debt** | **🚫 Blocked**. Update table, then proceed.

If no Progress table exists (older docs), infer completion and confirm with user.

### 2. Run tests (mandatory gate)

**Hard gate** — do not proceed until tests are green.

> [!TIP]
> **Skip if already green**: If the full test suite passed earlier in this conversation **and no code changes since**, skip re-running. Note prior result in walkthrough.

#### Discover how to run tests

// turbo

Check in order: Makefile targets → Docker/Lando commands → package.json scripts → direct commands (`php artisan test`, `phpunit`, etc.).

> [!IMPORTANT]
> PHP version mismatch? Find the containerized command — do not skip tests.

#### Run the tests

// turbo

Run full test suite and capture output.

#### Handle failures

- **Your changes**: Fix inline, re-run.
- **Pre-existing**: Fix inline, re-run. All failures must be resolved.
- **Unsure**: Check `git diff` for modified dependencies.

Repeat until green — zero failures required.

### 3. Run PHPStan (mandatory gate)

**Hard gate** — do not proceed until clean.

> [!TIP]
> **Skip if already green**: If `make phpstan` passed during `/implement` (step 8) **and no code changes since**, skip. Note in walkthrough.

// turbo

```bash
make phpstan
```

- **Your changes**: Fix inline, re-run.
- **Pre-existing**: Fix inline, re-run. All errors must be resolved.

Repeat until zero errors.

### 4. Test new code

Check whether new files/methods created during implementation lack test coverage. **Quality gate** — new code should not be closed without tests.

1. Identify new files from Progress table / walkthrough
2. Check for existing test files matching new classes
3. Write focused tests (happy path, edge cases, boundaries) for uncovered new code
4. Re-run test suite after adding tests

> [!TIP]
> **Skip if already covered**: If tests were written during implementation, verify they exist and move on.

> [!NOTE]
> New code only — extending coverage for pre-existing code is out of scope. File as debt if needed.

### 5. Code smell sweep

Quick scan of touched files and neighbors for smells: duplicated logic, dead code, wrong abstraction, magic values, missing interface methods.

For each smell: create a **separate debt doc** in `docs/` with `> Status: Debt`. One doc per issue.

> [!IMPORTANT]
> **Do NOT fix smells inline during close** — that's scope creep. File as debt, announce to user, move on.

If none found, skip.

### 6. Create the debt document (if needed)

When items were parked (Progress table) or smells filed (step 5):

// turbo

Create a sister doc with same slug + `-debt` suffix. Datetime prefix uses the **current time** (when debt is filed), not the parent doc's timestamp.

```markdown
# <Original Title> — Remaining Debt

> Created: YYYY-MM-DD HH:MM (local)
> Status: Draft
> Parent: <link to original document in finished/>

## Requirement

### <Item Name>

- **What**: <originally planned>
- **Why parked**: <reason>
- **Needed**: <what must happen>
- **Priority**: High | Medium | Low
```

Debt docs stay in active `docs/` — ready for `/plan`.

### 7. Append walkthrough to the source document

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
| [file.php](../path) | Description    |

### Decisions Made

1. **Topic**: answer — Rationale

### Open Debt

Items + link to debt doc (if created).
```

### 8. Finalize the original document

// turbo

- Set status to `Done`
- Add `> Finished: YYYY-MM-DD HH:MM (local)` line
- If debt doc created, add `> Debt: <link>` line

### 9. Move to finished

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

### 10. Report

Summarize: items completed, items as debt, decisions, file paths, test results, follow-ups.

#### Git commit message

Copy-paste ready conventional commit. Derive scope from doc slug:

```
feat(scope): summary

- One line per logical change
- Prefixed with -

Closes: docs/finished/YYYY-MM-DDTHHMM--slug.md
```

Types: `feat` | `fix` | `refactor` | `security`. Body: `-` per change. Footer: reference planning doc.
