---
description: "Initialize AI context — read all rules, evaluate skills, check parallelism gateway, confirm readiness"
---

# /init — Initialize AI Context

Load all governance context, verify tooling, and confirm readiness.

// turbo-all

## Steps

### Read every rule file

```bash
for f in .agent/rules/*.md; do echo "=== $(basename $f) ==="; cat "$f"; echo ""; done
```

After reading, list each filename and a one-line summary of what it requires.

### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

### Check parallelism gateway

```bash
.agent/bin/gemini-gateway --status
```

Report the result:

- `ok` → "Gateway available — will dispatch parallel work"
- `slow` → "Gateway slow — will limit to 1 dispatch at a time"
- `saturated` or error → "Gateway unavailable — will work single-threaded"

If the binary is missing, report: "Gateway not installed — parallelism disabled."

### Report to user

Confirm with a structured summary:

```
## Init Complete

### Rules
- [list each rule filename]

### Skills (14 active)
- [skill-name]: relevant / not relevant

### Parallelism
- Gateway: ok / slow / unavailable
- Model: will dispatch via `.agent/bin/gemini-gateway`

### Ready
Awaiting task.
```

### On every subsequent task

Before starting ANY work in this conversation, evaluate:

1. **Which skills match this specific task?** Read their full SKILL.md if not already read.
2. **Can this task be parallelized?** List sub-tasks, check independence, dispatch or state why not.
3. **Report both decisions** to the user before proceeding.
