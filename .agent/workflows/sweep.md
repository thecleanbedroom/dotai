---
description: Sweep the codebase for improvement opportunities — naming, structure, duplication, debt, friction — and produce a prioritized doc
---

# Codebase Sweep

Produce a prioritized list of improvement ideas based on what you observe in the codebase, your experience working in it, and standard best practices. The output is a doc that can be fed into `/plan` for implementation.

## What to Look For

1. **Naming inconsistencies** — methods, classes, files, or directories that don't follow the dominant convention in their area. Things that would make a newcomer guess wrong.

2. **Misplaced code** — logic living in the wrong layer or location. Controllers doing business logic, services doing presentation work, config in the wrong directory, utilities that belong in a shared location. Also look for flat directories with too many files that should be grouped into subdirectories by concern.

3. **Duplication** — similar patterns implemented differently across the codebase. Near-identical classes, copy-pasted logic, parallel hierarchies that could be unified.

4. **Unclear boundaries** — overlapping responsibilities between classes or modules. When it's not obvious which one owns a concern, or when a class does too many things.

5. **Friction points** — things that make working in the codebase harder than it needs to be. Non-obvious conventions, missing abstractions, code you have to read 5 files to understand.

6. **Unaddressed debt** — existing debt docs, TODO comments, skipped tests, or known issues that haven't been resolved.

7. **Standards gaps** — deviations from the project's own rules (`.agent/rules/`), framework best practices, or widely-accepted principles (SOLID, DRY, etc.).

## Steps

1. **Understand the project.** Read [PROJECT.md] and scan the codebase structure. Identify the platform, custom code areas, and conventions already in place.

2. **Load rules and skills.** Read all `.agent/rules/` files — these define the project's own standards and are the primary benchmark for the sweep. Evaluate `.agent/skills/` for relevant expertise (e.g., `clean-code`, `code-review-checklist`, language-specific skills) and read the `SKILL.md` for any that match. Use both as the lens for evaluating the codebase.

3. **Review knowledge and history.** Check knowledge items, conversation history, and `docs/finished/` for known debt, past decisions, and recurring friction.

4. **Walk the custom code.** Focus on application code, not framework scaffolding or vendor. For each area, evaluate against the "What to Look For" list above.

5. **Write the sweep report** to `docs/` following the project's doc naming convention. Structure:

   ```markdown
   # Codebase Sweep — <date>

   ## Summary

   Brief overview of findings and overall health impression.

   ## Findings

   ### [Category: e.g., Naming, Duplication, Misplaced Code]

   #### Finding title

   - **Where**: relative path(s)
   - **What**: what you observed
   - **Why it matters**: impact on readability, maintainability, or efficiency
   - **Suggested fix**: concrete, actionable suggestion
   - **Effort**: low / medium / high

   ...repeat for each finding...

   ## Prioritized Recommendations

   Top items ordered by impact-to-effort ratio. Each should be actionable as a `/plan` input.
   ```

6. **Present** the report for review via `notify_user`.

## Scoping

- If the user specifies a scope (module, directory, feature area), limit the sweep to that area.
- If no scope is given, sweep the entire custom codebase but keep findings actionable — don't produce a 200-item list. Focus on the highest-impact items.
- Aim for 10–20 findings max. Quality over quantity.
