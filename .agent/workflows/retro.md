---
description: "Audit agent effectiveness and capture development friction as actionable rule improvements"
---

# /retro — Agent Retrospective

Systematically evaluate how well the agent system performed in this conversation. Identify friction, missed opportunities, rule violations, and skill gaps. Produce concrete improvements — not observations.

**Input**: Optional description of specific friction (e.g., `/retro stop doing X`). Without input, performs a full audit of the current conversation.
**Output**: Updated rules, workflow patches, skill recommendations, and a summary report.

## Steps

### 1. Scan for friction

Review the full conversation for friction signals:

- **Revisions/corrections** — user had to correct the agent's approach
- **Rework** — code was written then reverted or rewritten
- **Wrong assumptions** — agent guessed instead of asking
- **Missed patterns** — existing code patterns were ignored
- **Wasted time** — sequential work that could have been parallel, unnecessary research, over-engineering

If the user provided a specific description with `/retro`, start with that and still scan for additional friction.

For each friction point, diagnose the **root cause**:

| Root Cause             | Fix Target                              |
| ---------------------- | --------------------------------------- |
| Missing rule           | Add to `retrospective.md`               |
| Ignored rule           | Strengthen wording or add to pre-flight |
| Wrong default          | Update rule with correct default        |
| Missing workflow step  | Patch the workflow                      |
| Missing skill guidance | Recommend skill addition                |
| Scope creep            | Add guard to relevant workflow          |

### 2. Check rule adherence

For each file in `.agent/rules/`, evaluate whether its requirements were followed:

- Read each rule file
- Check conversation evidence of compliance or violation
- Flag rules that were violated and note **why** (forgot? ambiguous wording? conflicting guidance?)

### 3. Check parallelism discipline

Evaluate parallelism for this conversation:

- Was parallelism evaluated before starting multi-item work?
- Were independent tasks dispatched via gateway?
- Were new-file phases dispatched (per retrospective rule)?
- If parallelism was skipped, was a valid reason given?

### 4. Check skill usage

Review work performed against available skills:

- Which active skills should have been read but weren't?
- Were skills read but their guidance not followed?
- Did the work need guidance that no skill provides? (candidate for addition)

### 5. Draft improvements

For each issue found, draft the concrete fix:

**New retrospective rules** — use this format:

```markdown
### <Summary of the rule — what to do>

**Pattern**: <What went wrong — one line>
**Rule**: <What to do instead — one to three lines>
**Workflow**: <path to workflow that needs improving, if traceable> (omit if no workflow involved)
```

**Workflow patches** — specify the exact file, step number, and new/revised text.

**Skill recommendations** — specify skill name from `.agent/skills-available/` and why it would help.

Before writing, read `.agent/rules/retrospective.md` — if a similar rule already exists, **revise and combine** rather than adding a duplicate.

### 6. Apply improvements

// turbo-all

1. Append new rules to `.agent/rules/retrospective.md`
2. Apply workflow patches to the relevant workflow files
3. Symlink recommended skills from `.agent/skills-available/` to `.agent/skills/`

### 7. Report

Summarize to the user:

| Category              | Count           |
| --------------------- | --------------- |
| Friction points found | N               |
| Rules violated        | N / N total     |
| Parallelism missed    | N opportunities |
| Skills underused      | N               |
| New rules added       | N               |
| Workflows patched     | N               |
| Skills added          | N               |

For each improvement made, show what was added/changed and why. Quote the exact text of new rules.
