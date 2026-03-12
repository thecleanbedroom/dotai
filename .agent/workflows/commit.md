---
description: "ONLY UPON REQUEST — Generate structured commit messages with git trailers for enhanced project memory. git push is FORBIDDEN."
critical: "NEVER commit without explicit user /commit invocation. NEVER git push under ANY circumstances."
---

# /commit — Enhanced Commit Message

> [!CAUTION]
> **NEVER commit unless the user EXPLICITLY invokes `/commit`.** Do not auto-commit after completing work. Do not proactively commit. Do not commit as part of another task. The ONLY time a `git commit` command should run is when the user says `/commit` or explicitly asks for a commit.
>
> **`git push` is ALWAYS FORBIDDEN.** Never run `git push` under any circumstances. Pushing is exclusively the user's action.

Generate a structured commit message with git trailers that enrich the project memory system.

// turbo-all

## Steps

### Stage all outstanding changes

`/commit` implies add + stage + commit. Stage everything the user hasn't explicitly excluded:

```bash
git add -A
```

### Analyze the staged changes

```bash
git diff --cached --stat
git diff --cached
```

### Generate the commit message

Using the diff context, generate a commit message with this structure:

```
<type>(<scope>): <short summary for humans reading git log>

<body - natural language explaining what changed and why>
<captures intent, decisions, tradeoffs — the stuff that evaporates when the session ends>

Type: feature|fix|refactor|config|debt|docs|test
Rationale: <why this approach was chosen — omit if obvious>
Rejected: <what was considered and not done — omit if nothing notable>
Fragile: <what's temporary, hardcoded, or likely to break — omit if solid>
Related: <slug or reference to related work — omit if standalone>
Confidence: high|medium|low
```

**Rules:**
- **NEVER commit unless the user explicitly invokes `/commit`** — do not auto-commit after completing work
- **NEVER `git push`** — `git push` is ALWAYS FORBIDDEN, no exceptions, no circumstances
- Summary line: imperative mood, ≤72 chars
- Body: wrap at 72 chars, explain the "why" not just the "what"
- `Type` trailer is always present
- Other trailers included only when relevant — a simple one-line fix only needs `Type: fix`
- `Confidence` reflects how confident you are in this change:
  - `high` = well-tested, clear requirements
  - `medium` = reasonable approach, some unknowns
  - `low` = quick fix, uncertain side effects
- The commit message captures your working context before it evaporates

### Commit

If the user invoked `/commit fast`, commit immediately without waiting for approval:

```bash
git commit -m "<generated message>"
```

Otherwise (plain `/commit`), show the generated message to the user. Apply only on approval.

```bash
git commit -m "<generated message>"
```
