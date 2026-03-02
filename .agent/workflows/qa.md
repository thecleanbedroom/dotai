---
description: Run a QA sweep against the codebase using core-quality-assurance rules
---

# QA Sweep

Run a structured quality assurance pass over the codebase (or a targeted scope) using the checks defined in `.agent/rules/core-quality-assurance.md`.

## Steps

### 0. Evaluate skills

// turbo

Scan installed skills and identify which ones are relevant to the task at hand:

```bash
for d in .agent/skills/*/; do echo "=== $(basename $d) ==="; head -5 "$d/SKILL.md" 2>/dev/null; echo ""; done
```

For each skill, decide: **relevant** or **not relevant** to this specific task. For every relevant skill, read its full `SKILL.md` and apply its guidance throughout the workflow. Briefly report which skills are active before proceeding.

1. **Load the QA rules**:

Read `.agent/rules/core-quality-assurance.md` in full. Every section is a checklist item.

2. **Determine scope**:
   - If the user specified files, directories, or a PR diff — scope to those.
   - If no scope given, focus on owned code paths (folders with `merge.json` or paths listed in `PROJECT.md` under "Owned Code Paths").
   - Never touch vendor, core, or contributed plugin files.

3. **Run baseline checks**:
   - Run the project's test suite and linters (use repo scripts, not hard-coded commands).
   - Note any failures — these must be resolved before proceeding.

4. **Execute each sweep in order**:

   a. **Code & Design** — single-responsibility, typed interfaces, dead code removal.

   b. **Security Sweep** — capability/nonce/authz, input sanitization, output escaping, prepared statements, secrets from env only. Scan recent changes for regressions. Add tests for gaps.

   c. **Performance Sweep** — N+1s, caching/transients, asset size/versioning, blocking external calls. Suggest concrete optimizations with measurements where possible.

   d. **TDD Sweep** — for any new or changed behavior, ensure tests exist. Write failing tests first, make them pass, refactor green.

   e. **Compatibility & Accessibility** — browser/runtime targets, WCAG AA contrast, `prefers-reduced-motion`.

   f. **Documentation & Ops** — update README/PROJECT if commands or behavior changed. Note migrations, env vars, release risks.

   g. **Observability** — structured logging, redacted sensitive fields, actionable error messages.

5. **Report findings**:

For each sweep, report:

- ✅ Items that pass
- ⚠️ Items that need attention (with file + line + suggested fix)
- ❌ Items that fail (with file + line + required fix)

6. **Cleanup** — remove temporary files/fixtures. Ensure git status is clean except intentional changes.

## Output

Provide a summary table:

| Sweep         | Status   | Issues |
| ------------- | -------- | ------ |
| Baseline      | ✅/❌    | count  |
| Code & Design | ✅/⚠️/❌ | count  |
| Security      | ✅/⚠️/❌ | count  |
| Performance   | ✅/⚠️/❌ | count  |
| TDD           | ✅/⚠️/❌ | count  |
| Compatibility | ✅/⚠️/❌ | count  |
| Docs & Ops    | ✅/⚠️/❌ | count  |
| Observability | ✅/⚠️/❌ | count  |

Then list each issue with its sweep, severity, file, line, and suggested fix.
