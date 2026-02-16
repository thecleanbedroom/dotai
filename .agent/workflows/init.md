---
description: "Initialize AI context — read .agent/index.md and confirm all pre-flight rules are loaded"
---

# /init — Initialize AI Context

Load the AI policy index and confirm all pre-flight rules are active.

// turbo-all

## Steps

1. **Read the index**:

```bash
cat .agent/index.md
```

Confirm you understand every instruction listed.

2. **Execute the index instructions** — follow each step in `.agent/index.md`:
   - Read every file in `.agent/rules/`
   - Scan `.agent/skills/` for applicable skills and read their SKILL.md files

3. **Report** — confirm to the user:
   - Rules loaded (list filenames)
   - Skills loaded (list filenames)
   - Ready to work
