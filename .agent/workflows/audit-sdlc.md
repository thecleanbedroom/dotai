---
description: "SDLC Meta — Audit all SDLC workflows for inconsistencies, stale references, and broken conventions"
---

# /sdlc-audit — Audit SDLC Workflow Consistency

Read every SDLC workflow file, check for inconsistencies, report findings, and fix them.

**Input**: None — reads all workflow files automatically
**Output**: Audit report with findings and fixes applied

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: `/capture` (self-contained) | `/hotfix` (fast-track)

**You are here**: `/sdlc-audit` — meta-workflow for auditing the SDLC itself

## Steps

### 1. Load all SDLC workflows

// turbo

Read every file in `.agent/workflows/`. Identify SDLC workflows (`SDLC Step` or `SDLC` in description). Currently: `plan.md` (1/3), `implement.md` (2/3), `close.md` (3/3), `capture.md` (Shortcut), `hotfix.md` (Shortcut).

### 2. Run consistency checks

Scan all SDLC workflow files for each check below. Report violations.

#### Check 1: Status lifecycle completeness

Valid lifecycle: `Draft → Planned → Approved → In Progress → Done → finished/`

| Workflow     | Accepts                     | Outputs       | Next accepts     |
| ------------ | --------------------------- | ------------- | ---------------- |
| `/plan`      | Draft, Debt, Planned, none  | `Approved`    | `/implement`: ✅ |
| `/implement` | Approved, In Progress       | `In Progress` | `/close`: ✅     |
| `/close`     | In Progress, Done (unfiled) | `Done`        | (terminal)       |
| `/capture`   | (self-contained)            | `Done`        | (terminal)       |
| `/hotfix`    | (self-contained)            | `Done`        | (terminal)       |

Verify: every workflow lists **all** statuses it might encounter, every "wrong status" guard names the **specific command** to run, outputs match next workflow's accepts. Flag mismatches.

#### Check 2: Frontmatter field references

Status reads must target `> Status:` (frontmatter), not body text. Flag `Check status` without `> Status:` or guard logic matching body text.

#### Check 3: Stale cross-references

Search for references to deleted/renamed workflows: `/work`, `/version`, `/fix`, `/review`, or any `/slash-command` without a matching file.

#### Check 4: SDLC Pipeline block

Every SDLC workflow needs `## SDLC Pipeline` with full path, lightweight path, and "You are here" marker. Verify all blocks match current workflow set.

#### Check 5: Description labels

YAML `description:` must start with `SDLC Step N/3 —` (main), `SDLC Shortcut —` (lightweight), or `SDLC Meta —` (audit). Verify numbering is sequential.

#### Check 6: Date/time format consistency

Filenames: `YYYY-MM-DDTHHMM`. Frontmatter: `YYYY-MM-DD HH:MM (local)`. Flag deviations.

#### Check 7: Template structure consistency

For workflows with doc templates (`/plan`, `/close`, `/capture`): verify frontmatter fields are consistent, same fields use same format, walkthrough sections are compatible.

#### Check 8: Guard redirect consistency

Every guard that rejects a doc must: name the exact command (`/implement`, not "correct workflow"), match `/plan`'s routing table, use "Tell user:" phrasing.

#### Check 9: turbo annotation consistency

Read-only steps → `// turbo`. Mutating/interactive steps → no turbo. `// turbo-all` makes individual `// turbo` redundant — flag overlap.

#### Check 10: Named cross-references

All step references must use **named refs** (e.g., `/plan`'s _Write the source document_), not step numbers. Applies to cross-file (`/plan` step N) and internal (`step 8`, `steps 8`) references. Exclude template labels like `Step 1/3`.

#### Check 11: Debt status guard

Debt docs use `> Status: Draft` — never `> Status: Debt`. The `🗑️ Debt` emoji is a Progress table status only. Search for `Status: Debt` (exclude guard routing logic and Progress table definitions).

#### Check 12: Link rebasing on move to finished

Every workflow that moves to `finished/` must rebase relative links (one directory level added). `/close` has rebase step — verify `/capture` and `/hotfix` reference it.

#### Check 13: DRY — pointer vs redefinition

Shortcut workflows must **reference** shared steps, not redefine inline. Canonical owners:

| Step                                                   | Owner    |
| ------------------------------------------------------ | -------- |
| Evaluate skills                                        | `/plan`  |
| Append walkthrough, Finalize, Move to finished, Report | `/close` |

Flag: inline bash snippets, templates, or process descriptions that duplicate a canonical step. Acceptable: one-liner references like "Follow `/close`'s _Move to finished_ step".

#### Check 14: Test and static analysis failure policy

All workflows with test/analysis gates must require **all failures fixed** — including pre-existing. Policy: zero failures, fix inline, re-run. Flag any "note but don't fix" language.

#### Check 15: Platform-agnostic language

Workflows must be framework-agnostic. Platform details belong in `.agent/rules/platform-*.md` or `language-*.md`. Flag hardcoded commands (`make phpstan`, `npm test`), language patterns (`.php`, `$this->`), or tool names (PHPStan, ESLint). Use generic: "test suite", "static analysis", "discover how to run tests".

### 3. Report findings

| #   | Check | File | Issue | Severity |
| --- | ----- | ---- | ----- | -------- |

Severity: **High** = wrong agent action. **Medium** = inconsistent info. **Low** = style.

### 4. Fix issues

// turbo

Fix all High and Medium. Present Low for user review. Re-run all checks to verify no regressions.

### 5. Dry-run walkthrough

**Run last**, after fixes. Simulate end-to-end, verifying each handoff:

1. User describes feature → `/plan` _Capture the intent_ → no doc, description input ✅
2. `/plan` _Ensure a source doc exists_ → stub with `Draft` status. Template matches `/implement` expectations?
3. `/plan` _Create the implementation plan artifact_ → has all sections `/implement` references?
4. `/plan` _Present for review_ / _Iterate until approved_ → user iterates via inline comments
5. `/plan` _Write the source document_ → full doc with Proposal, Reconciliation, Decisions
6. `/plan` _Mark as approved_ → `Approved`. `/implement` accepts? ✅
7. `/implement` _Load the document and plan_ → reads doc + artifact. Missing artifact handling?
8. `/implement` _Mark the source document_ → Progress table from Proposal phases
9. `/implement` _Report completion_ → invites review, Review rows tracked. `/close` accepts `In Progress`? ✅
10. `/close` _Load and verify_ → Progress table all terminal?
11. `/close` _Append walkthrough_ → references source doc data?
12. `/close` _Move to finished_ → finish-time prefix, rebase links ✅

#### Edge cases

- **Mid-plan interrupt**: Stub + artifact exist, status `Draft` → routes to "continue planning"?
- **Resume `/implement`**: `In Progress` → scans Progress table for `⬜ Ready`?
- **Debt flow**: Debt doc `Draft` → `/plan` accepts? `> Parent:` useful?
- **`/capture` bypass**: Produces `finished/`-ready doc? `/close` can't accidentally run on it?
- **Post-implementation review**: Changes tracked as Review rows? Status stays `In Progress`?

Flag any broken handoff or confused state.

### 6. Self-audit

Verify this audit workflow is current:

- Dry-run uses named step refs (not numbers)?
- Edge cases are realistic (no deleted concepts, covers new patterns)?
- Mechanical checks cover all active conventions?
- Workflow list matches actual files in `.agent/workflows/`?

Fix anything stale inline.

### 7. Summary

Report: total checks run, issues by severity, issues fixed, dry-run result, remaining items needing user decision.
