---
description: Audit .agent/rules/ and .agent/workflows/ for overlap, gaps, conflicts, staleness, and agent compliance
---

# Audit Agent System

Systematically evaluate every file in `.agent/rules/` and `.agent/workflows/` against the criteria below. Produce a scored report with concrete recommendations.

## Steps

1. **Inventory** — list every file in both directories with size and last-modified date:

```bash
ls -lh .agent/rules/ .agent/workflows/
```

2. **Read every file** — load all rule and workflow files so you have full context before evaluating.

3. **Promote retrospective entries** — review every entry in `.agent/rules/retrospective.md` for promotion. Do this BEFORE evaluating so evaluation checks the final state.

   For each entry, evaluate: validated across sessions? Destination file by topic (`core-workflow.md`, `core-testing.md`, `core-engineering.md`, `platform-laravel.md`, etc.)? Merge into existing section or append?

   For each entry ready for promotion:

   // turbo
   1. **Generalize** — strip project-specific references (class names, paths, APIs). If it can't be generalized, skip it.
   2. Add to the appropriate permanent file (condensed into the destination's style)
   3. Remove from `retrospective.md`
   4. Record in audit report

   **Skip**: entries from current session, too project-specific to generalize, or superseded by existing rules.

4. **Evaluate each category** below against the post-promotion state of all rule and workflow files. For each question, answer with ✅ (pass), ⚠️ (concern), or ❌ (fail) plus a brief explanation.

---

### Rules: Structure & Organization

| #   | Question                                                                                                                                           |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Overlap** — Do any two rule files cover the same topic? List specific duplicated guidance.                                                       |
| 2   | **Granularity** — Are any rule files too large (>5KB) to be a single rule? Should they be split?                                                   |
| 3   | **Naming** — Is the `core-*` / `language-*` / `platform-*` convention clear? Would an agent know where to look?                                    |
| 4   | **Missing coverage** — Are there languages, platforms, or concerns used in projects that have no rule file?                                        |
| 5   | **Misplaced rules** — Are there rules in one file that belong in a different file? Flag any rules whose topic doesn't match the file they live in. |
| 6   | **Condensation** — Could any rule files be merged without losing clarity? Flag files too thin to justify standalone existence.                     |

### Rules: Effectiveness

| #   | Question                                                                                         |
| --- | ------------------------------------------------------------------------------------------------ |
| 7   | **Actionability** — Is every rule concrete enough to act on? Flag any that are too vague.        |
| 8   | **Testability** — For each rule, could an agent verify it followed it? Flag unenforceable rules. |
| 9   | **Conflicts** — Do any rules contradict each other across files? Quote the conflicting lines.    |
| 10  | **Priority** — When rules conflict, is it clear which wins? Is priority documented?              |

### Workflows: Structure & Organization

| #   | Question                                                                                                   |
| --- | ---------------------------------------------------------------------------------------------------------- |
| 11  | **Overlap** — Do any two workflows cover the same task? List specific duplicated steps.                    |
| 12  | **Naming** — Are workflow filenames descriptive and consistent? Would a user know which to invoke?         |
| 13  | **Missing coverage** — Are there common agent tasks (debugging, refactoring, deployment) with no workflow? |
| 14  | **Descriptions** — Does every workflow have a YAML `description` that clearly explains when to use it?     |

### Workflows: Effectiveness

| #   | Question                                                                                                                             |
| --- | ------------------------------------------------------------------------------------------------------------------------------------ |
| 15  | **Frontmatter** — Does every workflow have valid YAML frontmatter with a `description` field? Flag missing or malformed frontmatter. |
| 16  | **Actionability** — Is every workflow step concrete enough to execute? Flag vague steps.                                             |
| 17  | **Turbo annotations** — Are `// turbo` annotations applied correctly? Read-only steps should have it; mutating steps should not.     |
| 18  | **Self-containment** — Can each workflow be executed with only its own instructions, or does it depend on undocumented context?      |

### Cross-Cutting

| #   | Question                                                                                                                                                                                                                                          |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 19  | **Rule/workflow boundary** — Rules duplicate what a workflow orchestrates? Rules = _what_; workflows = _how_.                                                                                                                                     |
| 20  | **Scope & portability** — References to specific repos, paths, or tools that won't exist in every project?                                                                                                                                        |
| 21  | **Conditional activation** — Platform/language rules document when to skip? Workflows scoped correctly?                                                                                                                                           |
| 22  | **Orphan references** — References to nonexistent files, workflows, or rules? Cross-check all internal references.                                                                                                                                |
| 23  | **Absolute paths** — Any `file://` URIs or absolute paths? All paths must be relative.                                                                                                                                                            |
| 24  | **Staleness** — Outdated rules/workflows (deprecated APIs, old patterns, stale cross-references)?                                                                                                                                                 |
| 25  | **Token budget** — Total size of all rule + workflow files in KB. Reasonable for agent context windows?                                                                                                                                           |
| 26  | **Pre-flight compliance** — Pre-flight rule clearly instructs to read all rule files? Gaps?                                                                                                                                                       |
| 27  | **Signal-to-noise** — Rules/workflows agents would ignore due to length, vagueness, or low relevance?                                                                                                                                             |
| 28  | **Sweep coverage** — All sweeps in `core-quality-assurance.md` map to a concrete declarative section?                                                                                                                                             |
| 29  | **Plugin sync** — `.agent/index.md` synced to plugin configs? Diff against: `.gemini/styleguide.md`, `.github/copilot-instructions.md`, `.windsurf/rules/rules.md`, `.continue/rules/rules.md`, `.cursor/rules/policy.mdc`, `.codex/config.toml`. |
| 30  | **File size** — `wc -c` all rule and workflow files. Flag any over **12,000 bytes** — condense without losing meaning.                                                                                                                            |

---

5. **Score** — assign an overall health score:

| Score | Meaning                           |
| ----- | --------------------------------- |
| A     | Clean — no ❌, ≤2 ⚠️              |
| B     | Healthy — no ❌, 3-5 ⚠️           |
| C     | Needs attention — 1-2 ❌ or 6+ ⚠️ |
| D     | Overhaul recommended — 3+ ❌      |

6. **Recommendations** — for every ⚠️ and ❌, propose a specific fix:
   - Files to merge, split, rename, or delete
   - Rules to reword, elevate, or remove
   - Missing rules or workflows to create
   - Sections to move from rules into workflows (or vice versa)
   - Retro entries promoted (with destination file)

## Output

```
## Agent System Audit — <date>

### Summary
- Rule files: <count>
- Workflow files: <count>
- Total size: <KB>
- Retro entries promoted: <count>
- Score: <A/B/C/D>

### Findings
<table of all 30 questions with ✅/⚠️/❌ and explanation>

### Retro Promotions
| Entry | Promoted To | Action |
| --- | --- | --- |
| <rule title> | <destination file> | Merged into § <section> |
| <rule title> | — | Skipped: <reason> |

### Recommendations
<numbered list of specific actions>
```
