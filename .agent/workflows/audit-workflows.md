---
description: Audit all workflows against core-workflow-authoring rules for structural quality
---

# /audit-workflows — Audit Workflow Structural Quality

Scan every workflow in `.agent/workflows/` against the rules in `.agent/rules/core-workflow-authoring.md`. Report and fix violations.

**Input**: None — reads all workflow files automatically
**Output**: Audit report with findings and fixes applied

// turbo-all

## Steps

### Load all workflows and authoring rules

Read `.agent/rules/core-workflow-authoring.md` to load the active ruleset.

Read every `.md` file in `.agent/workflows/`. For each, extract headings, step references, code blocks, turbo annotations, and descriptions.

### Extract structural data

Run this extraction script to produce machine-readable data for the mechanical checks. Review the output before proceeding.

```bash
cd .agent/workflows && python3 -c "
import os, re, json
from collections import defaultdict

data = {}
for fname in sorted(os.listdir('.')):
    if not fname.endswith('.md'): continue
    with open(fname) as f:
        content = f.read()

    # Frontmatter
    desc = ''
    fm = re.match(r'^---\s*\n(.*?)\n---', content, re.DOTALL)
    if fm:
        for line in fm.group(1).splitlines():
            if 'description:' in line.lower():
                desc = line.split(':', 1)[1].strip().strip('\"').strip(\"'\")[:100]

    # Headings
    headings = [m.group(1).strip() for m in re.finditer(r'^#{3,}\s+(.+)$', content, re.MULTILINE)]

    # Turbo
    has_turbo_all = '// turbo-all' in content
    turbo_lines = [i for i, l in enumerate(content.splitlines(), 1) if l.strip() == '// turbo']

    # Named cross-refs
    named_refs = re.findall(r'\x60/(\w[\w-]*)\x60(?:\'s|\u2019s)?\s+_([^_]+)_', content)

    # Step number refs (outside code blocks, inline code, and YAML frontmatter)
    stripped = re.sub(r'^---\s*\n.*?\n---', '', content, flags=re.DOTALL)
    non_code = re.sub(r'\x60\x60\x60.*?\x60\x60\x60', '', stripped, flags=re.DOTALL)
    non_code = re.sub(r'\x60[^\x60]+\x60', '', non_code)
    step_nums = re.findall(r'\bsteps?\s+(\d[\d,-]*)\b', non_code, re.IGNORECASE)

    # Numbered headings (N. prefix)
    numbered_headings = [h for h in headings if re.match(r'^\d+\.', h)]

    # Platform terms (outside code blocks)
    platform_hits = []
    for pat in [r'\bmake\s+(?:phpstan|test|lint|build|deploy|install|artisan[\w-]*)\b',
                r'\bnpm\s+(?:test|run)\b', r'\bcomposer\s+(?:install|require|update)\b',
                r'\bpytest\b', r'\bbundle\s+exec\b',
                r'\bPHPStan\b', r'\bESLint\b', r'\bRuboCop\b', r'\bPylint\b',
                r'\bLaravel\b', r'\bRails\b', r'\bDjango\b', r'\bReact\b']:
        for m in re.finditer(pat, non_code):
            platform_hits.append(m.group())

    # Classify steps for turbo audit
    step_classes = []
    for h in headings:
        hl = h.lower()
        safe = any(w in hl for w in ['read', 'load', 'scan', 'extract', 'verify', 'list', 'run test', 'clone', 'clean'])
        unsafe = any(w in hl for w in ['present', 'approve', 'review', 'ask', 'iterate', 'report'])
        step_classes.append({'name': h, 'safe': safe, 'unsafe': unsafe})

    data[fname] = {
        'desc': desc, 'headings': headings, 'has_turbo_all': has_turbo_all,
        'turbo_lines': turbo_lines, 'named_refs': [{'wf': r[0], 'step': r[1]} for r in named_refs],
        'step_nums': step_nums, 'numbered_headings': numbered_headings,
        'platform_hits': platform_hits, 'step_classes': step_classes
    }

print(json.dumps(data, indent=2))
"
```

This gives you structured data for every workflow. Use it to power the mechanical checks below.

### Run structural checks

Checks 1–3, 5–6, 8–10 are **mechanical** — run them against the extracted data. Checks 4 and 7 require **AI reasoning** — read the actual workflow content and compare semantically.

#### Check 1: Unique step headings

Every heading within a single workflow must be unique. Duplicate headings make cross-references ambiguous.

From the extraction data, flag any workflow where the `headings` array contains duplicates (case-insensitive).

#### Check 2: Global heading uniqueness for referenced steps

Across all workflows, collect step headings that are referenced by other workflows. If two different workflows define identically named steps, references become ambiguous. Flag collisions.

Exclude generic headings that are never cross-referenced (e.g., "Steps", "Output", "Summary").

#### Check 3: Step references use names, not numbers

From the extraction data, check the `step_nums` field for each workflow. Any non-empty value is a violation.

Exclude these false positives (the extraction script already strips code blocks):

- YAML `description` fields containing `Step N/3`
- `## Steps` section headers followed by numbered list items
- Example text describing what a violation looks like (e.g., inside this audit workflow)

#### Check 4: No inline redefinition of shared steps (AI reasoning)

**This check requires reading actual workflow content, not just extraction data.**

For each workflow that contains named cross-references (check the `named_refs` field), read both the referencing workflow and the target workflow. Verify the referencing workflow doesn't also contain an inline copy of the same logic (bash script, template, or process description).

The canonical owner is the workflow that defines the step in the most detail. Others should reference it with a one-liner.

#### Check 5: Platform-agnostic language

From the extraction data, check the `platform_hits` field. Flag any non-empty values.

Exclude: code blocks that are genuinely platform-agnostic infrastructure (e.g., `git clone`, `mkdir`, `python3 -c` for scripting utilities). The extraction script already filters to non-code sections.

Acceptable: generic terms like "test suite", "static analysis", "linter", "run the project's tests".

#### Check 6: Cross-reference resolution

For each named ref in the extraction data, verify the target heading exists in the target workflow's headings array. Flag dangling references.

#### Check 7: Canonical ownership consistency (AI reasoning)

**This check requires reading actual workflow content, not just extraction data.**

Scan for patterns where two or more workflows contain inline definitions for the same concern (e.g., "move to finished", "append walkthrough", "evaluate skills"). The canonical owner should have the full definition; others should reference it.

**Known canonical owners:**

| Concern                                                | Owner         |
| ------------------------------------------------------ | ------------- |
| Evaluate skills                                        | `/skills`     |
| Append walkthrough, Finalize, Move to finished, Report | `/close`      |
| Clone skills repo, Extract catalog                     | `/add-skills` |

Flag: parallel definitions of the same concern without a reference relationship.

#### Check 8: Frontmatter completeness

From the extraction data, flag any workflow where `desc` is empty.

#### Check 9: turbo annotation audit

From the extraction data, combine `has_turbo_all`, `turbo_lines`, and `step_classes` to evaluate:

- **Dangerous turbo** (High): `// turbo` on steps classified as `unsafe` (interactive/approval steps)
- **Redundant turbo** (Low): Individual `// turbo` when `// turbo-all` is set
- **Missing turbo-all** (Medium): All steps classified as `safe` but no `// turbo-all`
- **Missing individual turbo** (Low): Safe steps without `// turbo` in workflows that don't have `// turbo-all`

Report per workflow:

| Workflow | turbo-all? | Steps | Safe | Unsafe | Missing turbo | Redundant |
| -------- | ---------- | ----- | ---- | ------ | ------------- | --------- |

#### Check 10: Step numbering convention

From the extraction data, check the `numbered_headings` field. Any non-empty value is a violation — step headings should use descriptive names without number prefixes.

#### Check 11: Evaluate skills boilerplate

Every workflow that needs skill evaluation should reference `/skills`'s _Evaluate skills_ step with a one-liner. Flag any workflow that contains an inline copy of the evaluate-skills bash script or instructions instead of the reference.

### Report findings

| #   | Check | File | Issue | Severity |
| --- | ----- | ---- | ----- | -------- |
| 1   | Name  | file | desc  | H/M/L    |

Severity: **High** = agent would do the wrong thing (broken ref, dangerous turbo). **Medium** = DRY violation, inconsistency, or missing turbo-all. **Low** = style or suggestion.

### Fix issues

Fix all High and Medium. Present Low for user review. Re-run the extraction script and all checks to confirm no regressions.

### Summary

Report: total checks run, total workflows scanned, issues by severity, issues fixed, remaining items needing user decision.
