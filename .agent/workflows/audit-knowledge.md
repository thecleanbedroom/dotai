---
description: "Audit Knowledge Items for accuracy, relevance, staleness, and structural integrity"
---

# /audit-knowledge — Audit Knowledge

Systematically validate every Knowledge Item (KI) in the Antigravity knowledge system. Flag stale, inaccurate, duplicated, or contradictory entries. Outputs a health report with recommended actions.

**Input**: None — reads all KIs automatically
**Output**: Audit report with findings and actions taken

## Steps

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Inventory all KIs

// turbo

List every KI directory and load each `metadata.json`:

```bash
for d in ~/.gemini/antigravity/knowledge/*/; do
  echo "=== $(basename $d) ==="
  cat "$d/metadata.json" 2>/dev/null | head -20
  echo ""
done
```

Build a table: KI slug, title, summary, artifact count, reference count, last-modified date.

### Load all KI artifacts

// turbo

For each KI, read its artifacts. You need the full content to evaluate accuracy.

### Run validation checks

Evaluate each KI against the checks below. For each finding, note severity (**High** = actively misleading, **Medium** = stale/inaccurate, **Low** = style/structural).

---

#### Accuracy against current codebase

For each KI that references specific code patterns, files, classes, or architecture:

- Verify the referenced files/classes still exist
- Verify the described patterns still match reality
- Flag KIs where the codebase has diverged from the documented knowledge

#### Consistency with current rules

Compare each KI against `.agent/rules/` files:

- Flag KIs that contradict current rules
- Flag KIs that are fully absorbed into rules (redundant after retro promotion)
- Flag KIs that describe conventions now superseded by newer rules

#### Duplication across KIs

Compare KI titles and summaries pairwise:

- Flag KIs covering the same topic or concern
- Identify which is more comprehensive (keep one, retire the other)

#### Reference integrity

For each KI's `metadata.json` references:

- Conversation references: note if referenced conversation is very old (awareness only — conversations aren't deletable)
- File references: verify referenced files still exist
- Flag KIs with zero references (orphaned knowledge — where did it come from?)

#### Artifact integrity

For each KI's artifacts directory:

- Verify all files listed in `metadata.json` actually exist
- Flag empty or zero-byte artifacts
- Flag artifacts that are just stubs or placeholders

#### Staleness assessment

Evaluate temporal relevance:

- KIs about specific bugs/fixes: still relevant or one-time resolution?
- KIs about architecture: does the architecture still look like this?
- KIs about tooling/config: are the tools/versions still current?
- KIs about conventions: promoted to rules and no longer needed as KIs?

#### Signal-to-noise

Evaluate whether each KI adds value:

- Is the knowledge specific enough to be useful, or too generic?
- Would an agent benefit from this KI, or would it just add noise to context?
- Is the KI scoped correctly (not too broad, not too narrow)?

#### Cross-KI consistency

Check for KIs that give conflicting guidance:

- Two KIs recommending different approaches to the same problem
- KIs with contradictory architectural claims
- KIs that reference each other but have drifted apart

---

### Report findings

Write findings to an artifact file and present for review:

```markdown
# Knowledge Audit — <date>

## Inventory

| Metric          | Value           |
| --------------- | --------------- |
| Total KIs       | <count>         |
| Total artifacts | <count>         |
| Oldest KI       | <date> (<slug>) |
| Newest KI       | <date> (<slug>) |

## Findings

| Check | KI  | Issue | Severity | Action |
| ----- | --- | ----- | -------- | ------ |

## Health Summary

- ✅ Healthy: <count>
- ⚠️ Needs attention: <count>
- ❌ Should remove: <count>
```

Severity: **High** = actively misleading the agent. **Medium** = stale or inaccurate. **Low** = structural or style issue.

### Act on findings

For each finding, based on severity:

- **High (misleading)**: Recommend removal — stale knowledge is worse than no knowledge
- **Medium (stale)**: Recommend update or removal
- **Low (structural)**: Note for awareness, no immediate action

**CRITICAL: Never delete KIs without user approval.** Present all candidates and wait for explicit confirmation before invoking `/forget`.

### Self-audit

Verify this audit workflow is current:

- Check list matches KI directory structure (metadata.json, artifacts/, timestamps.json)?
- Checks cover all dimensions of KI quality?
- Actions reference correct workflows (`/forget`, `/remember`)?
