---
description: "Iterative sweep→implement loop — scans codebase, fixes findings, repeats until clean. Supply a target path."
---

# /sweeploop — Sweep & Fix Until Clean

Automated loop: sweep the target, implement all actionable findings, re-sweep, repeat until zero findings remain. No user approvals between iterations.

**Input**: Target path (directory or module to sweep).
**Output**: Final clean sweep report.

## Steps

### 1. Evaluate skills

Follow `/skills`'s _Evaluate skills_ step. Identify relevant skills once — they apply to all iterations.

### 2. Set iteration counter

Start at iteration 1. Cap at **10 iterations** to prevent infinite loops.

### 3. Loop: Sweep → Implement → Verify

Repeat the following until the sweep returns **zero actionable findings** or the iteration cap is reached:

#### 3a. Sweep (follow /sweep)

Run `/sweep` against the target path:

- Run QA baseline (`go vet`, `go test`, `go build` or platform equivalent)
- Structural scan using `/sniff` checklist
- Security, performance, docs checks
- Risk analysis with coverage data
- Write findings to sweep report artifact

**Classify each finding as:**
- **Actionable**: can be fixed now (any effort level that doesn't require fundamental redesign)
- **Debt**: requires architectural changes, integration tests needing new infrastructure, or user decisions — file as debt and skip
- **Accepted**: intentional patterns (e.g., `_ = godotenv.Load`, safe type assertions) — note as reviewed and skip

If **zero actionable findings** → stop looping, write final report, done.

#### 3b. Implement (follow /implement)

For all actionable findings from the sweep:

- Implement fixes — parallelize across files where possible
- Do NOT stop to ask for user approval between fixes
- Do NOT stop to ask for user approval between iterations

#### 3c. Verify

After implementing all fixes for this iteration:

// turbo-all

```bash
go vet ./... 2>&1
```

```bash
go test ./... -count=1 2>&1
```

```bash
go build -o /dev/null ./cmd/... 2>&1
```

If verification fails, fix the failure before proceeding to the next sweep iteration.

#### 3d. Increment counter

Increment iteration counter. If at cap (10), stop and report remaining findings.

### 4. Write final report

Update the sweep report artifact with:

- Final QA baseline (all green)
- Per-iteration summary (what was found and fixed)
- Any remaining debt items
- "Zero actionable findings" confirmation

### 5. Notify user

Present the final report via `notify_user`. Include iteration count and total fixes applied.
