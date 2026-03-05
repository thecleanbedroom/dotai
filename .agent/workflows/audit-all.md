---
description: Audit the full .agent/ system — rules, workflows, and knowledge — for overlap, gaps, conflicts, staleness, and agent compliance
---

# /audit-all — Audit Everything

Orchestrates the three sub-audits and produces a combined scored report. Each sub-audit writes its own artifact report.

## Steps

### Run sub-audits

Run each sub-audit in order. Each produces its own artifact report:

- `/audit-policy` — rules & policy health (structure, effectiveness, cross-cutting)
- `/audit-workflows` — workflow structural quality (mechanical + AI checks covering headings, cross-refs, turbo, circular deps, orphan refs, DRY, SDLC pipeline, doc templates)
- `/audit-knowledge` — KI accuracy, staleness, duplication, structural integrity

### Score

Combine findings from all three sub-audits and assign an overall health score:

| Score | Meaning                           |
| ----- | --------------------------------- |
| A     | Clean — no ❌, ≤2 ⚠️              |
| B     | Healthy — no ❌, 3-5 ⚠️           |
| C     | Needs attention — 1-2 ❌ or 6+ ⚠️ |
| D     | Overhaul recommended — 3+ ❌      |

### Recommendations

For every ⚠️ and ❌ across all sub-audits, propose a specific fix:

- Files to merge, split, rename, or delete
- Rules to reword, elevate, or remove
- Missing rules or workflows to create
- Sections to move from rules into workflows (or vice versa)

## Output

Write the combined summary to an artifact file and present for review. Use this template:

```markdown
# Agent System Audit — <date>

## Summary

| Metric          | Value     |
| --------------- | --------- |
| Score           | <A/B/C/D> |
| Rule files      | <count>   |
| Workflow files  | <count>   |
| Knowledge items | <count>   |
| Total size      | <KB>      |

## Sub-Audit Results

| Audit          | Findings         | Key Issues |
| -------------- | ---------------- | ---------- |
| Rules & Policy | <✅/⚠️/❌ count> | <summary>  |
| Workflows      | <✅/⚠️/❌ count> | <summary>  |
| Knowledge      | <✅/⚠️/❌ count> | <summary>  |

## Recommendations

<prioritized list of specific actions from all sub-audits>
```
