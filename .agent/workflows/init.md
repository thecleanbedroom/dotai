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

Follow /lib:#Evaluate Skills#.

### Check parallelism gateway

Call the gateway MCP tool:

```
gateway_status()
```

Report the result:

- `ok` → "Gateway available — will dispatch parallel work"
- `slow` → "Gateway slow — will limit to 1 dispatch at a time"
- `saturated` or error → "Gateway unavailable — will work single-threaded"

If the MCP server is not connected, report: "Gateway not available — parallelism disabled."

### Check project memory

Call the project-memory MCP tool:

```
project_memory_overview()
```

Report the result:

- `total_memories > 0` → "Memory available — <N> memories loaded, last build: <date>"
- `total_memories == 0` → "Memory empty — run `make build-memories` in `.agent/mcp/mcp-project-memory/` to populate"

If the MCP server is not connected, report: "Project memory not available — will rely on Knowledge Items only."

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
- Model: will dispatch via `gateway_dispatch` / `gateway_batch_dispatch` MCP tools

### Project Memory
- Status: <N> memories / empty / unavailable
- Last build: <date> or "never"

### Ready
Awaiting task.
```

### On every subsequent task

Before starting ANY work in this conversation, evaluate:

1. **Which skills match this specific task?** Read their full SKILL.md if not already read.
2. **Can this task be parallelized?** List sub-tasks, check independence, dispatch or state why not.
3. **Report both decisions** to the user before proceeding.
