---
description: "Generative sweep — propose how code SHOULD look, then diff against reality. Accepts a persona lens. Use with /personas for multi-perspective analysis."
---

# /sweep-design — Generative Codebase Sweep

Design-first analysis: consult project memory, propose how a best-in-class project would organize this code, then identify the gap with what exists. Not "find smells" but "design better, then diff against reality."

**Input**: Target path, optional `--work` modifier, optional checklist from caller.
**Output**: Findings with before/after proposals using `/lib`'s _Canonical Document Format_ item fields.

## Core Principles

1. **Generative, not evaluative.** Propose how code SHOULD look, then identify the gap.
2. **Architectural altitude.** Module boundaries, data flows, dependency graphs before function-level issues.
3. **Reference-based comparison.** Compare against well-maintained open source patterns.
4. **Before/after proposals.** Every finding includes what exists and a concrete sketch of what it should become.
5. **Honest but rigorous.** If no improvements are found, report clean — but document what was checked and how.

## Steps

### Resolve input

Follow `/lib`'s _Resolve Input_ step.

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Research

Follow `/lib`'s _Research Deep_ level for the target path. When called by `/personas`, this step is skipped (already done).

---

### Consult project memory

> [!IMPORTANT]
> Query the project memory MCP **before** analysis. Use it to understand what "better" looks like for THIS codebase specifically.

Query using the current focus:

1. **`search_project_memory_by_topic`** — search for focus areas (e.g., "error handling patterns", "module boundaries", "data flow")
2. **`search_file_memory_by_path`** — search for memories associated with files in the target path
3. **`project_memory_overview`** — understand the overall memory landscape

From the results, note:
- Established patterns and abstractions
- Past architectural decisions and their rationale
- Known debt and recurring friction
- Relationship mappings between components

These inform the reference design — the proposal should respect established decisions while improving on them.

---

### Propose reference design

The core generative step. After loading all context, propose how a best-in-class version of this project type would be organized.

**Prioritize architectural altitude** — ask "how should this system be organized" before "is this function clean":

1. **Module boundaries** — Are responsibilities cleanly separated? Would you draw the package/module lines differently?
2. **Data flows** — How does data move through the system? Are there unnecessary transformations, redundant passes, or unclear ownership?
3. **Dependency graph** — Do dependencies point in the right direction? Are there hidden couplings or circular references?
4. **Abstraction layers** — Are the right abstractions in place? Too many layers? Too few? Leaky boundaries?
5. **Error propagation** — How do errors flow? Is there a consistent strategy?

For each dimension, state a **structural assertion** — a concrete claim about how this code should be organized. Draw on knowledge of how well-maintained open source projects solve equivalent problems.

Express assertions as concrete claims, not vague principles:
- ✅ "The build package should not import from storage — it should receive a `JSONStore` interface"
- ❌ "Dependencies should be properly managed"

Use diagrams to illustrate gaps where the before/after is complex enough to warrant visualization.

### Gap analysis

Diff each structural assertion against the actual code. For each gap:

```markdown
#### Gap title

- **Assertion**: how it should be
- **Reality**: how it is (with file paths and line references)
- **Before**: current code sketch or structure diagram
- **After**: proposed code sketch or structure diagram
- **Why**: impact on the driving question
- **Priority**: High | Medium | Low
- **Effort**: Low | Medium | High
```

The before/after must be detailed enough that implementation is straightforward — not pseudocode or hand-waving, but a concrete sketch.

When no gaps are found for an assertion, note it as **verified** with evidence of what was checked.

---

### Execute the checklist

If a checklist is provided by the caller, work through each item **methodically** against the actual codebase:

- Read the relevant code for each checklist item
- Record a verdict (pass/fail/not-applicable) with evidence
- For failures, add to findings with before/after proposals

When no external checklist is provided, use `/sniff`'s _Smell checklist_ as default.

> [!IMPORTANT]
> Every checklist item must be addressed with evidence. "Looks fine" is not a verdict — cite the file, line, and pattern that satisfies the check.

### Do the work (when `--work` modifier is active)

When running in work mode:

- Fix actionable gaps directly after identifying them
- Follow skills guidance for clean, idiomatic changes
- Classify non-fixable gaps using `/lib`'s _Classification_
- Record what was changed and what remains

When running in doc mode: report findings only.

---

### QA verification

Follow `/lib`'s _QA Verification_ step.

---

### Risk analysis

Follow `/lib`'s _Risk Analysis_ step.

---

### Write findings

Write or return findings. Each finding uses the gap analysis format (assertion/reality/before/after). Order by architectural impact.

When a sweep completes with **zero gaps**, document the evidence:

```markdown
### Clean sweep evidence

| Assertion | Verified by | Evidence |
|-----------|------------|----------|
| Module boundaries are clean | Reviewed all package imports | No cross-boundary violations found |
| Dependencies point inward | Traced dependency graph | domain → config → git/llm → build → storage → server |
```

When running standalone, present via `notify_user`. When called by `/personas`, return findings for collection.

## Scoping

- If the user specifies a scope, limit analysis to that area.
- If no scope given, analyze the full custom codebase.
- Prioritize 5–10 architectural findings over 50 function-level ones.
- Risk analysis: top 10 methods above threshold.
