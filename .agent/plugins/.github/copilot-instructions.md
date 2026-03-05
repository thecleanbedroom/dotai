# CRITICAL: Non-Negotiable (Every Task)

1. **Parallelize first** — before starting ANY multi-item task, evaluate for parallelism. If work can be split into independent tasks (different files, no output dependency, no shared mutable state), it MUST be split and dispatched via the gateway. Work on your own tasks while agents run — never idle-wait. Validate results on completion.

```bash
run_command("echo '...' | .agent/bin/gemini-gateway --model <quick|fast|think|deep> --label 'description'")
```

2. **Read all rules** — READ every file in `.agent/rules/`. Treat each as a CRITICAL instruction that MUST be followed.

3. **Scan skills** — your available skills are listed in your system prompt under "Available skills". For each skill matching your current task, `view_file` its SKILL.md and apply its guidance. Do this in the same batch as reading rules.

4. **Never skip steps 1–3.** If you find yourself about to start coding without having evaluated parallelism, read rules, and scanned skills — STOP and complete this checklist first.
