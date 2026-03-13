# JSON-First Memory System — Detailed Design

## Overview

The memory system shifts from **SQLite-as-source** to **JSON-as-source**. Each memory is an independent JSON file. The SQLite database becomes a disposable runtime index, rebuilt on demand from JSON files. This enables multi-developer collaboration, git-native merge safety, and crash-resilient builds.

## 1. File Layout

```
project-root/
└── .agent/memory/
    └── data/
        ├── memories/              ← git-tracked, one JSON per memory
        │   ├── a1b2c3d4.json
        │   ├── e5f6a7b8.json
        │   └── ...
        ├── processed.json          ← git-tracked, sorted commit hashes
        └── project_memory.db      ← gitignored, ephemeral
```

### Why one file per memory?
- **Merge safety**: adding/deleting files never conflicts (unique UUIDs)
- **Git history**: each memory has its own creation/modification history
- **Selective review**: PRs show exactly which memories were added/removed
- **Crash resilience**: partial builds leave valid JSON files for completed work

## 2. Memory JSON Schema

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "type": "decision",
  "summary": "Switched to numeric confidence scores (0-100) instead of categories",
  "confidence": 78,
  "importance": 0.8,
  "source_commits": ["abc123def456"],
  "file_paths": ["src/models.py", "src/build.py"],
  "tags": ["confidence", "scoring", "refactor"],
  "created_at": "2026-03-12T22:00:00+00:00",
  "links": [
    {
      "target": "e5f6a7b8-1234-5678-abcd-ef1234567890",
      "relationship": "supersedes",
      "strength": 0.9
    }
  ]
}
```

| Field | Type | Source | Mutability |
|-------|------|--------|------------|
| `id` | UUID string | Generated at creation | Immutable |
| `type` | enum string | Pass 1 extraction | Immutable |
| `summary` | string | Pass 1, may be refined by pass 2 | Updated by synthesis |
| `confidence` | int (0-100) | Computed from structural signals | Immutable |
| `importance` | float (0-1) | Pass 1, may be adjusted by pass 2 | Updated by synthesis |
| `source_commits` | string[] | Pass 1 extraction | Immutable |
| `file_paths` | string[] | Pass 1 extraction | Immutable |
| `tags` | string[] | Pass 1 extraction | Immutable |
| `created_at` | ISO 8601 | Generated at creation | Immutable |
| `links` | object[] | Pass 2 synthesis | Appended by synthesis |

### Fields NOT in JSON (DB-only, ephemeral)
- `accessed_at` — tracks when MCP clients last read this memory
- `access_count` — how many times the memory has been retrieved
- `active` — always `true` for JSON files (inactive = file deleted)
- `updated_at` — tracked in git history naturally

> [!NOTE]
> JSON chosen over markdown+YAML frontmatter: these files are a data store
> accessed only via MCP, not human-edited. `json.loads()` is simpler and
> requires no additional dependencies.

## 3. processed.json Format

```json
[
  "0a1b2c3d4e5f6789abcdef0123456789abcdef01",
  "1234567890abcdef1234567890abcdef12345678",
  "2345678901abcdef2345678901abcdef23456789"
]
```

- Sorted JSON array of commit hash strings
- Pretty-printed with one hash per line for clean git diffs
- Duplicate entries are harmless (set semantics at read time)

## 4. Build Lifecycle

### Phase 1: Commit Discovery

```
Read processed.json → processed_set
git log --format=%H → all_commits
unprocessed = all_commits - processed_set
if empty → "no new commits", exit
```

> [!NOTE]
> Unlike `last_commit..HEAD`, this approach handles rebases, cherry-picks,
> and non-linear history correctly. Each commit is individually tracked.

### Phase 2: Extraction (Pass 1)

Same as current system. Fast model processes commit batches concurrently.

```
unprocessed commits → batch by token budget → concurrent LLM extraction
Result: list of raw Memory objects (in-memory, not yet persisted)
```

Each raw memory gets a UUID assigned immediately.

### Phase 3: Incremental Synthesis (Pass 2)

```
existing_memories = read all data/memories/*.json
new_memories = raw memories from phase 2

Prompt to reasoning model:
  "NEW memories: [full details of each new memory]
   EXISTING corpus: [compact: id + summary + type + tags for each]
   
   For each NEW memory, decide:
   - ACCEPT: worthy of keeping
   - REJECT: low quality or redundant
   
   For EXISTING memories, considering the new information:
   - DEACTIVATE: superseded by a new memory
   - UPDATE: importance/summary adjustment needed
   
   Create LINKS between new and existing memories."
```

**Why this is cheaper than full synthesis:**
- Existing memories sent as **compact summaries** (~50 tokens each vs ~200 full)
- Only new × existing comparisons, not N² over entire corpus
- Existing memories' inter-relationships already established

### Phase 4: Persist Results

```python
for memory in synthesis.accepted:
    write_json(f"data/memories/{memory.id}.json", memory)

for uuid in synthesis.deactivated:
    delete_file(f"data/memories/{uuid}.json")

for update in synthesis.updates:
    memory = read_json(f"data/memories/{update.id}.json")
    memory.importance = update.importance
    memory.summary = update.summary  # if refined
    write_json(f"data/memories/{update.id}.json", memory)

for link in synthesis.links:
    # Append link to the source memory's JSON
    memory = read_json(f"data/memories/{link.source}.json")
    memory.links.append({"target": link.target, ...})
    write_json(f"data/memories/{link.source}.json", memory)

# Update processed.json
append_and_sort(processed.json, newly_processed_hashes)
```

### Phase 5: Rebuild DB

```python
db = create_fresh_db()  # or truncate existing
for json_file in glob("data/memories/*.json"):
    memory = Memory.from_json(json_file)
    db.insert(memory)
db.rebuild_fts_index()
db.store_fingerprint(hash_of_json_file_list)
```

## 5. MCP Server Startup

```
1. Does project_memory.db exist?
   NO  → rebuild from JSON (phase 5 above)
   YES → check fingerprint against current JSON files
         STALE → rebuild from JSON
         FRESH → serve from DB
```

**Fingerprint**: SHA-256 of sorted JSON filenames concatenated. Cheap to compute. Stored in a `db_meta` table.

## 6. Edge Cases

### Build interrupted mid-extraction
- Some commits extracted, others not
- processed.json NOT yet updated (update happens after all phases complete)
- Next build re-processes all unprocessed commits (correct behavior)

### Build interrupted mid-synthesis
- Raw memories extracted but not yet persisted as JSON
- processed.json NOT yet updated
- Next build re-extracts and re-synthesizes (correct, idempotent)

> [!TIP]
> For long builds, consider batching processed.json updates after each
> extraction batch completes, so interrupted builds don't redo everything.

### Two developers process overlapping commits
- Both create JSON files with different UUIDs for same commit's memories
- processed.json merges cleanly (both add same hash)
- Next synthesis may detect duplicates and deactivate one
- Worst case: slightly redundant memories until next synthesis

### Memory references a deleted memory via link
- Memory A links to memory B, but B's JSON was deleted (deactivated)
- DB import skips broken links silently
- Harmless — the link just doesn't appear in queries

### Very large corpus (1000+ memories)
- Incremental synthesis only sends compact summaries of existing memories
- ~50 tokens per existing memory × 1000 = ~50K tokens (well within context)
- Full details only for new memories (typically 5-20 per build)

## 7. Cost Analysis

| Operation | Model | Frequency | Est. Cost |
|-----------|-------|-----------|-----------|
| Pass 1 (extraction) | Free/cheap model | Per new commit batch | FREE or ~$0.001/batch |
| Pass 2 (synthesis) | Reasoning model | Per build (incremental) | ~$0.01-0.05 |
| DB rebuild | None (local) | On MCP startup if stale | $0 |

Incremental synthesis is significantly cheaper than full synthesis because:
- Existing corpus sent as compact summaries only
- Only new memories need full reasoning
- Existing links/relationships are preserved, not recomputed
