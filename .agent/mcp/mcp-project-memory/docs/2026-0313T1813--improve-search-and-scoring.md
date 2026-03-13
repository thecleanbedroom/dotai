# Improve Search & Scoring Quality

> Created: 2026-03-13 18:13 (local)
> Status: Draft

## Requirement

### Add date-range filtering to topic search

- **What**: Add optional `since` and `until` parameters to `search_project_memory_by_topic`
- **Where**: `src/server.py`, `src/memory/stores.py`
- **Why**: Cannot filter by time range — had to manually sort dates to reconstruct the audit system timeline. Time filtering is critical for "what changed recently" queries.
- **How**: Add `since`/`until` string params (ISO date), add `created_at >= ? AND created_at <= ?` clauses to the FTS query in `MemoryStore.search()`
- **Priority**: High
- **Effort**: Low

### Improve extraction prompt to collapse bulk operations

- **What**: Add guidance to the extraction prompt telling the LLM to produce ONE memory per bulk operation, not one per file
- **Where**: `src/config/prompts/prompt_extract_system.md`
- **Why**: ~50% of the 168 memories are fragments of skill addition/removal. 17+ memories for what should be 2 ("added initial skill library", "pruned to essentials"). This is the #1 noise source.
- **How**: Add a "BULK OPERATIONS" rule section telling the LLM to collapse file additions/deletions into a single high-level memory when the commit adds/removes many files of the same kind
- **Priority**: High
- **Effort**: Low

### Calibrate importance scoring in extraction prompt

- **What**: Add importance scoring guidance with calibrated examples to the extraction prompt
- **Where**: `src/config/prompts/prompt_extract_system.md`
- **Why**: Importance scores are oddly flat (75-90 for everything). A bulk skill deletion and a core architectural decision both scored ~80-90. Without calibration examples, the LLM defaults to mid-high for everything.
- **How**: Add an "IMPORTANCE CALIBRATION" section with anchored examples: 90-100 = core architecture decisions, 60-80 = meaningful changes, 30-50 = routine housekeeping, 0-20 = trivial
- **Priority**: High
- **Effort**: Low

### Widen confidence score distribution

- **What**: Adjust the confidence scoring thresholds in `MemoryFactory` to produce a wider, more discriminating distribution
- **Where**: `src/build/memory_factory.py`, `src/config/internal.py`
- **Why**: Most memories cluster at 40-65 confidence. The thresholds reward "has lots of files" and "has a long summary" instead of actual evidence quality. A single-commit bulk operation touching 50 files gets max confidence.
- **How**: Reduce the weight of file count (cap at 15 instead of 30), increase weight of multi-commit evidence (single commit = 5 instead of 8), add a penalty for very high file counts (bulk indicator)
- **Priority**: Medium
- **Effort**: Low

### Add `exclude_tags` filter to topic search

- **What**: Add an optional `exclude_tags` parameter to `search_project_memory_by_topic`
- **Where**: `src/server.py`, `src/memory/stores.py`
- **Why**: Tags from skill-topic content (e.g. "architecture" tag on a skill-addition memory) pollute topic searches. Being able to exclude `skill-removal`, `skill-library` etc. would improve relevance.
- **How**: Add `exclude_tags` list param, filter out memories whose `tags` JSON contains any excluded tag
- **Priority**: Medium
- **Effort**: Low
