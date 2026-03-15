---
description: "Audit workflows for structural quality, cross-references, DRY, SDLC pipeline integrity, and file hygiene"
---

# /audit-workflows — Audit Workflow Quality

Scan all `.agent/workflows/` files for structural issues, DRY violations, SDLC pipeline consistency, and broken conventions.

**Input**: None — reads all workflow files automatically
**Output**: Audit report with findings and fixes applied

// turbo-all

## Steps

### Load rules and workflows

Read `.agent/rules/core-workflow-authoring.md` and every `.md` file in `.agent/workflows/`.

### Extract structural data

Run this for machine-readable data powering the mechanical checks:

```bash
cd .agent/workflows && python3 -c "
import os,re,json
data={}
for fn in sorted(f for f in os.listdir('.') if f.endswith('.md')):
  c=open(fn).read()
  fm=re.match(r'^---\s*\n(.*?)\n---',c,re.DOTALL)
  desc=''
  if fm:
    for l in fm.group(1).splitlines():
      if 'description:' in l.lower(): desc=l.split(':',1)[1].strip().strip('\"\'')[:100]
  hd=[m.group(1).strip() for m in re.finditer(r'^#{3,}\s+(.+)$',c,re.MULTILINE)]
  s=re.sub(r'^---\s*\n.*?\n---','',c,flags=re.DOTALL)
  nc=re.sub(r'\x60{3}.*?\x60{3}','',s,flags=re.DOTALL)
  nc=re.sub(r'\x60[^\x60]+\x60','',nc)
  data[fn]={'desc':desc,'headings':hd,'has_turbo_all':'// turbo-all' in c,
    'turbo_lines':[i for i,l in enumerate(c.splitlines(),1) if l.strip()=='// turbo'],
    'named_refs':[{'wf':r[0],'step':r[1]} for r in re.findall(r'\x60/(\w[\w-]*)\x60(?:\'s|\u2019s)?\s+_([^_]+)_',c)],
    'step_nums':re.findall(r'\bsteps?\s+(\d[\d,-]*)\b',nc,re.I),
    'numbered_headings':[h for h in hd if re.match(r'^\d+\.',h)]}
print(json.dumps(data,indent=2))
"
```

### Run checks

**(M)** = mechanical (extraction data). **(AI)** = requires reading content and applying judgment.

---

#### Heading uniqueness (M)

- **Within file**: flag duplicate headings (case-insensitive). Exempt code blocks.
- **Across files**: flag cross-referenced headings defined in multiple workflows. Exempt generic non-target headings (Steps, Summary).

#### Step references use names (M)

From `step_nums`. Non-empty = violation. Exclude YAML `Step N/3` descriptions.

#### Cross-reference resolution (M)

For each `named_refs`, verify target heading exists in target workflow. Flag dangling.

#### Frontmatter and labels (M)

- Non-empty YAML `description` required
- SDLC workflows: description starts with `SDLC Step N/3 —`, `SDLC Shortcut —`, or `SDLC Meta —`
- Status reads target `> Status:` frontmatter. Filenames: `YYYY-MM-DDTHHMM`. Dates: `YYYY-MM-DD HH:MM (local)`.

#### Platform-agnostic language (AI)

Read each workflow's full content — including code blocks, examples, and inline references. Evaluate: **does this workflow contain platform-specific or language-specific commands, code, examples, or terminology that would make it non-portable?**

Flag any workflow that hardcodes framework-specific commands (`make artisan-test`), language-specific code (PHP parsers, Python scripts for app logic), tool-specific patterns (`PHPUnit`, `Jest`), or framework names used prescriptively (not as meta-examples).

Exempt: meta-examples showing what a pattern *looks like* (e.g., audit pattern lists), and workflows whose purpose is inherently platform-specific (e.g., if a `platform-laravel.md` workflow existed). Use generic alternatives: "run the test suite", "run static analysis", "follow project conventions".

#### Turbo annotations (M + AI)

Mechanical detection from `turbo_lines` and `has_turbo_all`:

- **Redundant turbo** (Low): individual `// turbo` lines present alongside `// turbo-all`
- **Missing turbo-all** (Low): no `// turbo-all` when it could be appropriate

AI evaluation for safety:

- **Dangerous turbo** (High): For each step with `// turbo`, read the step's actual content and evaluate whether auto-execution is safe. A step is unsafe if it presents results for user review, asks for approval, requires iteration on feedback, or makes destructive changes without guardrails. Do not rely on step name keywords alone — evaluate from context.

#### Step numbering convention (M)

From `numbered_headings`. Non-empty = violation — use descriptive names.

#### Skills loading (M)

Every applicable workflow references `/skills`'s _Evaluate skills_ as a one-liner before substantive work. Flag inline copies or late placement.

#### Circular dependencies (M)

Build dependency graph from `named_refs`. Run cycle detection. Classify:

- **ℹ️ Mutual reference** (2-node, e.g. A→B→A): expected, informational
- **❌ True recursive loop** (3+ nodes, no redirect-only edges): High severity

#### Orphan references (M)

Scan rules + workflows for references to nonexistent files, workflows, or rules. Flag unresolved.

#### Absolute paths (M)

Flag `file:///` URIs or absolute paths. All must be relative. Exclude descriptive mentions.

#### File size (M)

Flag any rule or workflow file over **12,000 bytes** — condense without losing meaning. Exempt `lib.md` and `lib-*.md` (reference-only files loaded on demand, not at startup).

#### Canonical ownership and DRY (AI)

Verify referencing workflows don't inline canonical logic. Owners:

| Concern | Owner |
|---------|-------|
| Evaluate skills | `/skills` |
| Walkthrough, Finalize, Move to finished, Debt doc, Report | `/close` |
| Clone skills repo, Extract catalog | `/skillsfinder` |
| Smell checklist, Logging format | `/sniff` |
| Document format, Resolve input, Requirement item format, Research, Tracing, Classification, QA Verification, Risk Analysis, Persona definitions | `/lib` |

#### Document format and input resolution (AI)

All doc-creating workflows must use `/lib`'s _Canonical Document Format_. Entry-point workflows must reference _Resolve Input_. Verify:

- No inline templates (only `/lib` defines the template)
- Requirement items use standard fields (What, Where, Why, How, Priority, Effort)
- Debt docs reference `/close`'s _Create debt doc_, use `> Status: Debt`
- Source docs include `> Status:` and `> Created:` frontmatter
- Every status value written is accepted by a downstream routing table
- No format mismatches between doc creators and consumers

#### SDLC pipeline integrity (AI)

**Status lifecycle**: `Draft → Planned → Approved → In Progress → Done → finished/`

| Workflow | Accepts | Outputs | Next |
|----------|---------|---------|------|
| `/plan` | Draft, Debt, Planned, none | `Approved` | `/implement` ✅ |
| `/implement` | Approved, In Progress | `In Progress` | `/close` ✅ |
| `/close` | In Progress, Done (unfiled) | `Done` | (terminal) |
| `/capture` | any (via _Resolve Input_) | `Done` | (terminal) |
| `/hotfix` | any (via _Resolve Input_) | `Done` | (terminal) |

Verify: outputs match next accepts, redirects name exact commands, every SDLC workflow has `## SDLC Pipeline` with "You are here".

#### Link rebasing and test policy (AI)

- Workflows moving to `finished/` must rebase relative links. `/capture` and `/hotfix` reference `/close`'s step.
- Test/analysis gates require **all failures fixed**. Flag "note but don't fix" language.

---

### Report findings

Write to artifact: `# Workflow Audit — <date>` with table: Check | File | Issue | Severity.

Severity: **High** = wrong action. **Medium** = DRY/inconsistency. **Low** = style.

### Fix issues

Fix High and Medium. Present Low for review. Re-run checks to confirm no regressions.

### Dry-run walkthrough

Simulate full SDLC path verifying each handoff:

| Step | Check |
|------|-------|
| `/plan` _Resolve input_ → doc with `Draft` | Doc created using canonical format? |
| `/plan` _Create the implementation plan artifact_ | Has all sections `/implement` needs? |
| `/plan` _Present for review_ → _Iterate until approved_ → _Write the source document_ → _Mark as approved_ | Status = `Approved`, `/implement` accepts? |
| `/implement` _Resolve input_ → _Load the document and plan_ | Reads doc + artifact correctly? |
| `/implement` _Mark the source document and initialize progress tracking_ → _Report completion_ | `/close` accepts `In Progress`? |
| `/close` _Resolve input_ → _Load and verify_ | Progress all terminal? |
| `/close` _Append walkthrough to the source document_ → _Move to finished_ | Links rebased? |

**Edge cases**: mid-plan interrupt (stub + artifact, `Draft`), resume `/implement` (`In Progress`, scans `⬜ Ready`), debt flow (`Debt` → `/plan`), `/capture` bypass (finished-ready?), **sweep→hotfix→close** (standard doc throughout, no orphan).

### Self-audit

Verify: dry-run uses named refs, edge cases realistic, checks cover active conventions, workflow list matches files.

### Summary

Report: total checks, workflows scanned, issues by severity, fixed, remaining.
