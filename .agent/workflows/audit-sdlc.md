---
description: "SDLC Meta — Audit all SDLC workflows for inconsistencies, stale references, and broken conventions"
---

# /sdlc-audit — Audit SDLC Workflow Consistency

Read every SDLC workflow file and check for inconsistencies. Report findings and fix them.

**Input**: None — reads all workflow files automatically
**Output**: Audit report with findings and fixes applied

## SDLC Pipeline

**Full path**: `/plan` → `/implement` → `/close`
**Lightweight**: `/capture` (self-contained — for ad-hoc fixes)

**You are here**: `/sdlc-audit` — meta-workflow for auditing the SDLC itself

## Steps

### 1. Load all SDLC workflows

// turbo

Read every file in `.agent/workflows/`. Identify which are SDLC workflows (have `SDLC Step` or `SDLC` in their description). Currently:

- `plan.md` (Step 1/3)
- `implement.md` (Step 2/3)
- `close.md` (Step 3/3)
- `capture.md` (Shortcut)
- `hotfix.md` (Shortcut)

### 2. Run consistency checks

For each check below, scan all SDLC workflow files and report any violations.

#### Check 1: Status lifecycle completeness

The valid status lifecycle is:

```
Draft → Planned → Approved → In Progress → Done → finished/
```

Verify:

- Every workflow that checks status lists **all** statuses it might encounter (not just the ones it accepts)
- Every "wrong status" guard names the **specific command** to run (no vague "tell the user the correct workflow")
- The status a workflow **outputs** matches the status the **next** workflow **accepts**

| Workflow     | Accepts               | Outputs       | Next step accepts |
| ------------ | --------------------- | ------------- | ----------------- |
| `/plan`      | Draft, Planned, none  | `Approved`    | `/implement`: ✅  |
| `/implement` | Approved, In Progress | `In Progress` | `/close`: ✅      |
| `/close`     | In Progress           | `Done`        | (terminal)        |
| `/capture`   | (self-contained)      | `Done`        | (terminal)        |

Flag any mismatches.

#### Check 2: Frontmatter field references

Every place a workflow reads document status must specify `> Status:` line (frontmatter), not body text. Search for:

- `Check status` without `frontmatter` or `> Status:`
- Any guard logic that could match body text instead of frontmatter

#### Check 3: Stale cross-references

Search all SDLC workflows for references to workflows that **don't exist**. Check:

- `/work` (deleted)
- `/version` (deleted)
- `/fix` (renamed to `/capture`)
- `/review` (merged into `/plan`)
- Any other `/slash-command` references that don't match a file in `.agent/workflows/`

#### Check 4: SDLC Pipeline block

Every SDLC workflow must have a `## SDLC Pipeline` section with:

- Full path: `/plan` → `/implement` → `/close`
- Lightweight: `/capture`
- "You are here" marker with the current workflow bolded

Verify the pipeline block exists and matches the current workflow set. If a new workflow was added, all pipeline blocks need updating.

#### Check 5: Description labels

Every SDLC workflow's YAML `description:` must start with:

- `SDLC Step N/3 —` for the 3 main steps (check numbering is correct)
- `SDLC Shortcut —` for `/capture`
- `SDLC Meta —` for this audit workflow

Verify the step numbers are sequential and correct (1/3, 2/3, 3/3).

#### Check 6: Date/time format consistency

All datetime formats must match: `YYYY-MM-DD HH:MM (local)`. Search for any deviations:

- `YYYY-MM-DDTHHMM` in filenames (correct)
- `YYYY-MM-DD HH:MM (local)` in frontmatter (correct)
- Any other format (flag)

#### Check 7: Template structure consistency

For workflows that have doc templates (`/plan`, `/close`, `/capture`), verify:

- Frontmatter fields are consistent (Created, Status, Finished, Parent, Debt, etc.)
- Same fields use the same format across workflows
- Walkthrough sections in `/close` and `/capture` use compatible structures

#### Check 8: Guard redirect consistency

Every guard that rejects a doc should:

- Name the exact command to run (e.g., "Run `/implement`" not "use the correct workflow")
- Match the status-to-command mapping from `/plan`'s routing table
- Use consistent phrasing ("Tell user:" pattern)

#### Check 9: turbo annotation consistency

Check that `// turbo` annotations are used consistently:

- Steps that only read files should have `// turbo`
- Steps that modify files or ask the user should NOT have `// turbo`

### 3. Report findings

Present findings as a table:

| #   | Check            | File         | Issue                                  | Severity |
| --- | ---------------- | ------------ | -------------------------------------- | -------- |
| 1   | Status lifecycle | implement.md | Guard doesn't mention `Planned` status | Medium   |
| ... | ...              | ...          | ...                                    | ...      |

Severity levels:

- **High**: Could cause the agent to take the wrong action (wrong redirect, missing guard)
- **Medium**: Inconsistent wording or missing information
- **Low**: Style or formatting inconsistency

### 4. Fix issues

// turbo

Apply fixes for all High and Medium issues. Present Low issues for user review.

After fixing, re-run checks 1-9 to verify no regressions.

### 5. Dry-run walkthrough

**Run this last**, after all mechanical fixes are applied. The dry run validates the _fixed_ workflows actually work as a coherent system — not just that fields are correct, but that the flow makes sense.

Mentally simulate a realistic scenario end-to-end, verifying each handoff:

#### Scenario: A user wants to add a feature

1. **User says**: "I want to add caching to the sync pipeline"
2. **`/plan` step 1**: Agent reads the request. No existing doc — identified as description input. ✅ or flag issue.
3. **`/plan` step 3**: Agent creates a stub doc in `docs/`. What status? What sections? Does the stub template match what `/implement` later expects to read?
4. **`/plan` step 4**: Agent creates implementation plan artifact. Does the template have all sections `/implement` will reference?
5. **`/plan` steps 5-6**: User iterates via inline comments. Does the artifact template support the commenting patterns described?
6. **`/plan` step 7**: User approves. Agent writes full source doc. Does the written doc have the sections that `/implement` needs (Proposal with phases, Reconciliation, Decisions)?
7. **`/plan` step 8**: Status set to `Approved`. Does `/implement` accept `Approved`? ✅
8. **`/implement` step 1**: Agent reads the `Approved` doc. Does it also load the artifact? What if the artifact was from a previous conversation and no longer exists?
9. **`/implement` step 2**: Agent adds Progress table. Does the phase list come from the source doc's Proposal section? Is there a clear mapping?
10. **`/implement` step 7**: All phases done. Agent says "Run `/close`". Status is `In Progress`. Does `/close` accept `In Progress`? ✅
11. **`/close` step 1**: Agent reads `In Progress` doc. Checks Progress table — all terminal? Proceeds.
12. **`/close` step 4**: Appends walkthrough. Do the walkthrough sections reference data that exists in the source doc (phases, files, decisions)?
13. **`/close` step 6**: Moves to `finished/`. Filename gets finish-time prefix. Clean.

#### Edge cases to check

- **Mid-conversation interrupt during `/plan`**: Only a stub doc + artifact exist. User starts new conversation and says `/plan` on the stub. Does the status (`Draft`) correctly route to "continue planning"? Does the agent know to check for an existing artifact?
- **Resuming `/implement`**: Status is `In Progress`, Progress table exists. Does `/implement` step 1 correctly scan the table and find the next `⬜ Ready` item?
- **Debt flow**: `/close` creates debt doc with status `Draft`. User runs `/plan` on debt doc. Does `/plan` accept `Draft`? (Should proceed with planning.) Does the `> Parent:` link provide useful context?
- **`/capture` bypass**: Does `/capture` produce a doc that looks like it came through the full pipeline? Can `/close` be accidentally run on a `/capture` doc?

Flag any handoff that doesn't work or any state where the agent would be confused about what to do next.

### 6. Self-audit

The audit must also audit itself. After running all checks and fixes, verify:

- **Step references**: Does the dry-run scenario reference the correct step numbers in each workflow? (e.g., if `/plan` was reordered, step numbers in the scenario must match.)
- **Edge case relevance**: Are the edge cases in step 5 still realistic? Remove any that reference deleted concepts, add any for newly introduced patterns.
- **Check coverage**: Do the 9 mechanical checks cover all conventions currently in use? If a new convention was introduced (e.g., a new frontmatter field, a new status), add a check for it.
- **Workflow list**: Does step 1's workflow list match the actual files in `.agent/workflows/`?

If anything is stale, fix it inline.

### 7. Summary

Report:

- Total checks run (mechanical + dry-run + self-audit)
- Issues found (by severity)
- Issues fixed
- Dry-run result: clean handoff or issues found
- Any remaining items that need user decision
