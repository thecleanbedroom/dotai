---
description: "Evaluative sweep — checklist-based analysis for smells, QA, and risk. Accepts a persona lens. Use with /personas for multi-perspective analysis."
---

# /sweep-eval — Evaluative Codebase Sweep

Checklist-driven analysis: read code, apply smell checks, run QA verification, and score risk. Finds what's wrong against established standards.

**Input**: Target path, optional `--work` modifier.
**Output**: Findings using `/lib`'s _Canonical Document Format_ item fields.

## Steps

### Resolve input

Follow `/lib`'s _Resolve Input_ step.

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Research

Follow `/lib`'s _Research Deep_ level for the target path. When called by `/personas`, this step is skipped (already done).

---

### Structural scan

Walk the custom code (not framework/vendor). Apply `/sniff`'s _Smell checklist_ systematically. Evaluate for:

- SOLID principle violations (SRP, OCP, LSP, ISP, DIP)
- DRY violations and duplication
- Pattern and anti-pattern detection (appropriate idioms for the platform)
- Error handling taxonomy and consistency
- Naming, readability, and clarity
- Any issue surfaced by `/sniff` checklist categories

Record each finding:

```markdown
#### Finding title

- **What**: what you observed
- **Where**: relative path(s)
- **Why**: impact on readability, maintainability, or correctness
- **How**: concrete, actionable suggestion
- **Priority**: High | Medium | Low
- **Effort**: Low | Medium | High
```

### Do the work (when `--work` modifier is active)

When running in work mode (via `/personas --work` or standalone `--work`):

- Fix actionable findings directly after identifying them
- Classify non-fixable findings using `/lib`'s _Classification_
- Record what was changed and what remains

When running in doc mode (no `--work`): report findings only.

---

### QA verification

Follow `/lib`'s _QA Verification_ step.

---

### Risk analysis

Follow `/lib`'s _Risk Analysis_ step.

---

### Write findings

Write or return findings. Each finding uses the standard item fields. Order by impact-to-effort ratio.

When running standalone, present via `notify_user`. When called by `/personas`, return findings for collection.

## Scoping

- If the user specifies a scope, limit the sweep to that area.
- If no scope given, sweep the entire custom codebase but keep findings actionable.
- Aim for 10–20 structural findings max. Quality over quantity.
- Risk analysis: show all methods above threshold, prioritize top 10.
