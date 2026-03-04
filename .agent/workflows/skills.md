---
description: Evaluate installed skills for relevance to the current task
---

# Evaluate Skills

Scan installed skills and identify which ones are relevant to the task at hand.

// turbo-all

## Steps

### Evaluate skills

```bash
for d in .agent/skills/*/; do echo "=== $(basename $d) ==="; head -5 "$d/SKILL.md" 2>/dev/null; echo ""; done
```

For each skill, decide: **relevant** or **not relevant** to this specific task. For every relevant skill, read its full `SKILL.md` and apply its guidance throughout the workflow. Briefly report which skills are active before proceeding.
