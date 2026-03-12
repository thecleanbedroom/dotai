# Workflow Rules

Optimize for correctness, minimalism, and developer experience.

## Operating Principles

- **CRITICAL — Git discipline**: `git commit` is FORBIDDEN unless the user explicitly invokes `/commit` or asks for a commit. `git push` is ALWAYS FORBIDDEN — never push under any circumstances. No auto-commits after finishing work. No proactive commits. No commits as part of another task. Each commit is a separate, user-initiated action. Violating this is a hard failure.
- Correctness over cleverness; prefer boring, readable solutions.
- Small, simple, elegant change that works. If it feels hacky, reassess. Don't refactor adjacent code unless it reduces risk.
- Leverage existing patterns before introducing new abstractions.
- Prove it works — validate with tests/build/lint, not "seems right."
- Be explicit about uncertainty; propose safest next step when you can't verify.

## Orchestration

- **Plan mode** for non-trivial tasks (3+ files, multi-component, architectural, production-impacting). Include verification steps in the plan. Stop and re-plan when new information invalidates assumptions.
- **Subagents**: one focused objective per subagent with a concrete deliverable. Merge outputs into actionable synthesis before coding.
- **Incremental delivery**: thin vertical slices → implement → test → verify → expand. Use feature flags/config switches when feasible.
- **Self-improvement**: after corrections, add to `LESSONS.md` (failure mode, signal, prevention rule). Review at session start.
- **Verification before done**: evidence required (tests, lint, build, logs). Ask: "Would a staff engineer approve this diff?"
- **Bug fixing**: just fix it — check logs, errors, failing tests, then resolve. Reproduce → isolate → root-cause → fix → regression test → verify. Zero context switching required from the user.

## Task Management

Include verification tasks. Define acceptance criteria. Mark progress. Capture checkpoint notes. Add results section when done. Update `LESSONS.md` after corrections.

- Enforce naming conventions at write-time. When augmenting an existing file, check if its filename follows the project's naming convention — don't defer to creation-time only.
- **Real-time progress tracking**: update progress **immediately after completing each step** — this is part of the step's work, not a close-time cleanup task. This applies to:
  - Source doc Progress tables (mark phase ✅ Done)
  - Brain task artifacts (mark checklist item `[x]`)
  - Task boundary status/summary

  Do NOT batch progress updates or defer them until the end.

## Communication

- Lead with outcome/impact, not process. Reference concrete artifacts.
- Ask questions only when blocked; batch them with recommended defaults.
- State inferred assumptions. Show verification story (what ran, outcome).
- When the user describes a fix but the location is ambiguous (could be client-side or server-side, caller or callee), ask which layer should own the logic before writing code.
- No busywork updates — checkpoint only on scope changes, risks, or decisions needed.
- When a design decision has multiple valid approaches, present 2-3 concrete options with tradeoffs in the first draft. Let the user pick rather than guessing and iterating.

## Context Management

- Read before write — find authoritative source of truth first. Prefer targeted reads.
- Keep working notes in planning docs; compress when context grows large.
- Prefer explicit names and direct control flow. Control scope creep — log follow-ups as TODOs.
- When you spot code that looks wrong during unrelated work, create a tracking doc (debt/TODO) rather than fixing inline (scope creep) or just mentioning it in chat (gets lost).
- **Doc link portability**: all file references in `docs/` must use **relative paths** (e.g., `[file.php](../laravel/path/to/file.php)`), never absolute `file:///` URIs. Docs are checked into source control and must resolve for all developers.

## Error Recovery

- **Stop-the-line**: on unexpected failures, stop features → preserve evidence → re-plan.
- **Triage**: reproduce → localize → reduce → fix root cause → guard with test → verify e2e.
- **Convention-first debugging**: when a class exists but doesn't appear to take effect, verify the resolution/discovery mechanism works (e.g., convention-based loading, auto-discovery) before investigating explicit registration. Check assumptions before adding complexity.
- **Safe fallbacks**: prefer "safe default + warning" over silent failure.
- **Rollback**: keep changes reversible (flags, isolated commits). Ship risky changes disabled-by-default.
- **Instrumentation**: add logging only when it reduces debugging time; remove temp debug output.
