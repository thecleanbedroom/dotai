# Ignored Paths

> Created: 2026-03-13 11:02 (local)
> Status: Done

## Requirement

### Path Exclusion During Build

- **What**: Allow users to configure paths that should be excluded from memory extraction
- **Where**: `config.py`, `git.py`, `build.py`
- **Why**: Two initial commits contain 20M chars each — 98% from `.agent/skills/` (benchmark datasets, XML schemas, package-lock). This wastes tokens and produces irrelevant memories. `.agent/memory/data` should never be self-referencing.
- **How**: Add `MEMORY_IGNORE_PATHS` config with sensible defaults. Filter at the git diff level (cheapest) and on existing memories during DB rebuild (drop-out).
- **Priority**: High
- **Effort**: Low

### Drop-Out on Config Change

- **What**: When ignored paths change, existing memories referencing only ignored paths should be deactivated
- **Where**: `build.py` (DB rebuild step)
- **Why**: User adds a path to ignore list → old memories about that path should disappear from search results
- **How**: During `_rebuild_db`, skip memories whose `file_paths` are all in the ignore set
- **Priority**: Medium
- **Effort**: Low
