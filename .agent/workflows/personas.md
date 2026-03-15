---
description: "Run any sweep workflow through 10 personas with iteration. Usage: /personas <workflow> [--work] @<target>"
---

# /personas — Multi-Perspective Orchestrator

Run any workflow through `/lib`'s 10 personas with mandatory iteration until clean. Two modes control output format.

**Input**: `<workflow> [--work] @<target>` — e.g., `sweep @src/` or `sweep --work @src/`
**Output**: Clean codebase + report (both modes iterate until clean)

> [!IMPORTANT]
> **No delegation.** Do NOT dispatch persona work to MCP subagents (`gateway_dispatch`, `gateway_batch_dispatch`). All analysis and fixes must be performed directly. The persona loop requires full context continuity across iterations.

## Modes

| Mode | Behavior | Output |
|------|----------|--------|
| **Doc mode** (default) | Iterate until clean — find, fix, re-sweep | Aggregated findings doc presented at the end |
| **Work mode** (`--work`) | Iterate until clean — find, fix, re-sweep | Walkthrough of changes presented at the end |

Both modes use the **same iteration loop**, the **same exit condition**, and run **autonomously without pausing**. The only difference is output format.

## Steps

### Research & Skills

Follow `/lib`'s _Research Deep_ level for the target path. Run **once** before the persona loop — all personas share the same loaded context.

> [!IMPORTANT]
> **Skills are MANDATORY.** After research, follow `/skills` to scan available skills and `view_file` on every SKILL.md that matches the target codebase's language, platform, or the sweep type being performed. Build a **matched skills list** (e.g., `clean-code`, `golang-pro`, `uncle-bob-craft`). This list is injected into each persona's perspective preamble below. The inner workflow's own "Evaluate skills" step is skipped (already done here).

**Print matched skills** immediately after building the list:

```
--- Matched Skills ---
• clean-code — Clean Code principles (naming, functions, comments)
• golang-pro — Go 1.21+ modern patterns and idioms
• uncle-bob-craft — SOLID, clean architecture, design principles
---
```

### The Iteration Loop

> [!CAUTION]
> **MANDATORY ITERATION RULE**: The loop can ONLY exit when a COMPLETE iteration (all personas evaluated) produces **zero findings AND zero changes**. Any findings or changes in any iteration — even a single one — forces another full iteration. There is NO shortcut. Cap at **10 iterations** to prevent infinite loops.

```
iteration = 0
loop:
  iteration++
  iteration_findings = 0
  iteration_changes = 0

  for each persona in /lib's Persona Definitions (in order):
    compose perspective (see below)
    run inner workflow (see below)
    record persona_findings, persona_changes
    iteration_findings += persona_findings
    iteration_changes += persona_changes

  if iteration_findings == 0 AND iteration_changes == 0:
    EXIT — codebase is clean
  else:
    log: "Iteration {iteration}: {iteration_findings} findings, {iteration_changes} changes — re-sweep MANDATORY"
    if iteration >= 10:
      EXIT — max iterations reached, report remaining findings as debt
    goto loop
```

**You MUST maintain these counters explicitly.** At the end of each iteration, state the totals and whether re-sweep is triggered. Do NOT skip the count check.

#### Compose perspective

Follow `/lib`'s _Perspective Composition_ to compose a context-aware preamble for this persona. Use `/lib`'s _Context Reordering_ to direct attention based on the persona's Reading Order dimension.

**Inject matched skills**: Include the matched skills list from Research in the preamble. Reference each skill by name (e.g., "Apply `/clean-code` naming rules, `/golang-pro` idioms, and `/uncle-bob-craft` design principles"). This ensures skill guidance is actively part of every persona's analysis context.

#### Print perspective (MANDATORY)

> [!IMPORTANT]
> **You MUST print the composed perspective before running the inner workflow.** This makes persona analysis visible and auditable. Use this exact format:

```
═══ Persona {N}/10: {Persona Name} ═══

Perspective:
{The full composed preamble — 2-3 paragraphs from Perspective Composition}

Skills applied: {comma-separated matched skills list}
Entry point: {the concrete entry point being traced}
Driving question: "{the persona's driving question}"
═══════════════════════════════════════
```

This block must appear in the output for every persona in every iteration. Do NOT abbreviate or skip it even if the persona produces zero findings.

#### Run the inner workflow

Follow the specified workflow's steps with the composed perspective as context. The workflow reads the perspective and naturally focuses its analysis accordingly.

In **work mode**: the workflow implements changes directly.
In **doc mode**: the workflow reports findings, then the agent fixes actionable items before continuing.

#### Collect results

After each persona completes, record to the running totals:
- `persona_findings`: count of actionable items identified
- `persona_changes`: count of code changes applied

### After each persona (both modes)

#### Classify

Classify findings using `/lib`'s _Classification_.

#### Fix actionable findings

For all actionable findings:
- Follow `/skills` to identify relevant skills
- Implement changes — parallelize across files where possible
- Re-read every modified file and its neighbors for introduced issues
- Do NOT stop for user approval between changes within an iteration

#### Verify

- Follow `/lib`'s _QA Verification_
- If verification finds new issues, address them before proceeding
- Add the fixes to `iteration_changes`

### Exit check (MANDATORY — runs after every complete iteration)

> [!CAUTION]
> **This check is NON-NEGOTIABLE.** You must execute it after every iteration, no exceptions.

**Check**: `iteration_findings == 0 AND iteration_changes == 0`

- **YES**: Clean sweep confirmed. Proceed to _Write report_.
- **NO**: Log the totals. **Start a new full iteration.** Re-read any modified files. Run all personas again from the top.

### Write report

Create the report with:
- Final QA baseline (all green)
- Per-iteration summary: which personas found what, what was changed
- Per-persona verdict (zero findings confirmed in final clean iteration)
- Remaining debt items (if max iterations hit)
- Total iteration count and total changes applied

**Doc mode**: Present via `notify_user` with the aggregated findings doc.
**Work mode**: Present via `notify_user` with the walkthrough.
