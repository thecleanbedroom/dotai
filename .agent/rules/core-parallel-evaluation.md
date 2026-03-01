---
description: "CRITICAL: All work MUST be parallelized. Split into gateway agents via run_command whenever possible."
---

# Parallel Evaluation

> [!CAUTION]
>
> CRITICAL: All work MUST be parallelized. Evaluate every task for parallelism before starting. If it can be split, it MUST be split.

This applies to **all work** — implementation, edits, analysis, research, testing, verification. You are the orchestrator: dispatch, work in parallel, validate results.

## Mechanism

`run_command` dispatching to `.agent/bin/gemini-gateway` is the ONLY parallel mechanism.

```bash
# Single dispatch — agent reads source and writes files directly
run_command("echo '...' | .agent/bin/gemini-gateway --model <lite|quick|fast|think|deep> --label 'description'")

# Batch dispatch — multiple jobs from JSON stdin
run_command("echo '[{\"model\":\"fast\",\"prompt\":\"...\",\"label\":\"job-A\"},{\"model\":\"fast\",\"prompt\":\"...\",\"label\":\"job-B\"}]' | .agent/bin/gemini-gateway --batch")
```

## Parallelism Decision

1. **List** all discrete tasks (e.g., "update 5 classes" = 5 tasks)
2. **Check independence** — parallel only if: different files, no output dependency, no shared mutable state
3. **Batch** independent tasks into parallel groups; sequence dependent ones
4. **Dispatch** batch → work on your own tasks → validate → dispatch next batch

### Common Patterns

| Parallel ✅                    | Sequential ❌                 |
| ------------------------------ | ----------------------------- |
| Edit different files           | Edit same file                |
| Write impl + tests (from spec) | Add method + test that method |
| Research topic A + B           | Update interface + consumers  |
| Spot-check N items (read-only) | Task B depends on A's output  |

## Orchestrator Role

1. **Evaluate** parallelism graph before work starts
2. **Dispatch** independent tasks to gateway
3. **Work** on your own tasks (never idle-wait)
4. **Review** agent output via `git diff` and `git status` — agents write files directly
5. **Fix** minor issues inline; on retry, **improve the prompt or gateway first** (max 2 retries, then do it yourself)
6. **Never cancel** a running job — the model may still be working. Wait for completion.

> [!IMPORTANT]
> Agents write files directly to the codebase. Review via `git diff`, not by parsing stdout.

## Model Selection

| Tier    | Use for                                           |
| ------- | ------------------------------------------------- |
| `lite`  | Spot-checks, single reads, config lookups         |
| `quick` | Config edits, one-liners, small analysis          |
| `fast`  | Code generation, tests, refactoring, log analysis |
| `think` | Multi-file refactors, complex validation          |
| `deep`  | Architecture review, complex reasoning            |

> 5 tiers = capability selection. Jobs run serially (one at a time).

## Dispatch Modes

### Single dispatch

```bash
echo 'Write tests for MyService.php' | \
  .agent/bin/gemini-gateway --model fast --label 'MyService-tests'
```

### Batch dispatch

Multiple independent jobs in one call. Each job runs through the queue sequentially.

```bash
echo '[
  {"model":"fast", "prompt":"Extract MetafieldWriter from ShopifyService.php", "label":"extract-metafields"},
  {"model":"fast", "prompt":"Extract InventoryManager from ShopifyService.php", "label":"extract-inventory"}
]' | .agent/bin/gemini-gateway --batch --cwd /path/to/codebase
```

### Prompt tips

- Tell the agent which files to read first (exact paths)
- Tell the agent which files to write (exact paths)
- Be specific about namespace, conventions, and what NOT to modify
- Agents write files directly — no need for "output to stdout" instructions
- For convention-sensitive work (tests, configs, extensions), tell the agent to
  **read existing examples first** to discover project conventions (directory structure,
  base classes, naming, patterns) rather than describing them in the prompt. The agent
  learns better from real files than from prompt descriptions, and conventions you
  forget to mention won't be missed.

## Health Check

```bash
.agent/bin/gemini-gateway --status
```

`ok` → dispatch freely · `slow` → 1 at a time · `saturated` → do it yourself

## Timeout Discipline

- Never manage gateway timeouts from the orchestrator. The gateway handles its own timeout (killing a working model is worse than waiting). Just dispatch and check status.

## New-File Dispatch Default

- During parallelism triage, any phase that creates a **new file** with no dependency on a prior phase's output is a gateway candidate. The "quick enough" instinct is a trap — small tasks are ideal for dispatch precisely because they complete quickly and free you to focus on modification phases that require sequential reasoning. Default to dispatching, not inlining.

## When NOT to Split

Same file edits, output dependencies, singular tasks (one grep/one edit), gateway saturated, gateway binary not installed — proceed single-threaded without parallelism reporting.

## Mandatory Reporting

> [!IMPORTANT]
> Always report parallel usage to user.

- **On dispatch**: count, task descriptions, model tier, what you're doing meanwhile
- **On complete**: each result (pass/fail), `git diff` review summary
- **On skip**: why (shared files, dependencies, <30s task, saturated)
