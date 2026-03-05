---
description: Audit .agent/ rules for overlap, gaps, conflicts, and effectiveness, plus cross-cutting system health
---

# /audit-policy — Audit Rules & Policy

Evaluate every file in `.agent/rules/` and `.agent/workflows/` for structural health, effectiveness, and cross-cutting consistency.

## Steps

### Inventory

// turbo

List every file in both directories with size:

```bash
ls -lh .agent/rules/ .agent/workflows/
wc -c .agent/rules/*.md .agent/workflows/*.md | sort -n
```

### Read every file

// turbo

Load all rule and workflow files so you have full context before evaluating.

### Promote retrospective entries

Follow `/retro`'s _Promote entries_ step. Do this BEFORE evaluating so evaluation checks the final state.

### Evaluate

For each question below, answer with ✅ (pass), ⚠️ (concern), or ❌ (fail) plus a brief explanation.

---

#### Rules: Structure & Organization

| Question                                                                                                                                           |
| -------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Overlap** — Do any two rule files cover the same topic? List specific duplicated guidance.                                                       |
| **Granularity** — Are any rule files too large (>5KB) to be a single rule? Should they be split?                                                   |
| **Naming** — Is the `core-*` / `language-*` / `platform-*` convention clear? Would an agent know where to look?                                    |
| **Missing coverage** — Are there languages, platforms, or concerns used in projects that have no rule file?                                        |
| **Misplaced rules** — Are there rules in one file that belong in a different file? Flag any rules whose topic doesn't match the file they live in. |
| **Condensation** — Could any rule files be merged without losing clarity? Flag files too thin to justify standalone existence.                     |

#### Rules: Effectiveness

| Question                                                                                         |
| ------------------------------------------------------------------------------------------------ |
| **Actionability** — Is every rule concrete enough to act on? Flag any that are too vague.        |
| **Testability** — For each rule, could an agent verify it followed it? Flag unenforceable rules. |
| **Conflicts** — Do any rules contradict each other across files? Quote the conflicting lines.    |
| **Priority** — When rules conflict, is it clear which wins? Is priority documented?              |

#### Cross-Cutting

| Question                                                                                                                                                                                                                                                                                                                |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Rule/workflow boundary** — Rules duplicate what a workflow orchestrates? Rules = _what_; workflows = _how_.                                                                                                                                                                                                           |
| **Scope & portability** — References to specific repos, paths, or tools that won't exist in every project?                                                                                                                                                                                                              |
| **Conditional activation** — Platform/language rules document when to skip? Workflows scoped correctly?                                                                                                                                                                                                                 |
| **Staleness** — Outdated rules/workflows (deprecated APIs, old patterns, stale cross-references)?                                                                                                                                                                                                                       |
| **Token budget** — Total size of all rule + workflow files in KB. Reasonable for agent context windows?                                                                                                                                                                                                                 |
| **Pre-flight compliance** — Pre-flight rule clearly instructs to read all rule files? Gaps?                                                                                                                                                                                                                             |
| **Signal-to-noise** — Rules/workflows agents would ignore due to length, vagueness, or low relevance?                                                                                                                                                                                                                   |
| **Sweep coverage** — All sweeps in `core-quality-assurance.md` map to a concrete declarative section?                                                                                                                                                                                                                   |
| **Plugin sync** — `.agent/index.md` is the source of truth. Verify it's synced to `AGENTS.md` (root) and all plugin configs in `.agent/plugins/`: `.gemini/styleguide.md`, `.github/copilot-instructions.md`, `.windsurf/rules/rules.md`, `.continue/rules/rules.md`, `.cursor/rules/policy.mdc`, `.codex/config.toml`. |

---

### Report findings

Write findings to an artifact file and present for review:

```markdown
# Rules & Policy Audit — <date>

## Summary

| Metric                 | Value   |
| ---------------------- | ------- |
| Rule files             | <count> |
| Workflow files         | <count> |
| Total size             | <KB>    |
| Retro entries promoted | <count> |

## Findings

<table of all checks with ✅/⚠️/❌ and explanation, grouped by section>

## Retro Promotions

| Entry        | Promoted To        | Action                  |
| ------------ | ------------------ | ----------------------- |
| <rule title> | <destination file> | Merged into § <section> |

## Recommendations

<prioritized list of specific actions>
```
