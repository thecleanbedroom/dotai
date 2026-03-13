# Project Memory System — v1 Specification

## Purpose

Reduce iterations to senior-quality code by giving AI coding agents (Claude Code / Opus via Antigravity) persistent, queryable project knowledge that would otherwise only exist in a senior developer's head.

## Core Philosophy

- **Commits are the fundamental unit of memory.** Every commit is a memory event. The richer the commit, the richer the memory. But even a bare "hotfix" with a diff produces something.
- **Code is the source of truth.** Memory is supplementary context — the _why_ behind the code, the rejected approaches, the debt, the constraints. The code itself is always authoritative.
- **High confidence or silence.** The system only stores what it has strong evidence for. It never guesses. When Opus queries something the system doesn't know about, it returns nothing. Being wrong is worse than being silent.
- **Memories are hints, not authority.** Every memory carries provenance (source commits, dates). Opus should verify against the actual code before relying on a memory. Memory reduces discovery time; it doesn't replace reading the code.
- **Graceful degradation.** Rich structured commits → rich memories. Bare commits with just diffs → thin but still useful memories. No commits about an area → silence. The system works on any git repo from day one with zero setup.

## Design Decisions & Rejected Approaches

This section captures the reasoning behind key architectural choices made during design. These are the "memories" of building the memory system itself.

### Decision: Commits as the fundamental memory unit (not docs, not a separate memory store)

**Rationale:** We explored several approaches for where memories originate. Originally considered indexing a docs folder as the primary source, but realized commits are the one artifact guaranteed to exist in any project. Docs require workflow discipline. Commits always happen — you can't ship code without them. Even a bare "hotfix" with a garbage message still has a diff that means something.

**Rejected: Docs folder as primary source.** Docs are great when they exist but can't be the foundation because developer discipline varies. They become a supplemental layer that enriches memories when available.

**Rejected: Separate memory store that the AI writes to directly during development.** This was the initial instinct (inspired by the Google Always-On Memory Agent). Problem: it creates a second source of truth that can drift from the code. It also requires the AI to explicitly decide "this is worth remembering" during development, which adds friction and is unreliable. By deriving memory from commits, the system captures what actually happened rather than what the AI thought was important.

**Rejected: Real-time memory capture during coding sessions.** Conversational context between the developer and AI is valuable (failed attempts, rationale discussions) but ephemeral by nature. Rather than trying to capture it in real-time, we capture it at commit time — the AI writes a rich commit message while it still has that context. The commit is the developer's approval checkpoint ("this is real, this is the direction we're going").

### Decision: Git trailers for structured commit metadata (not JSON, not YAML, not custom format)

**Rationale:** We need structured data in commits that's parseable by the build system but also human-readable in `git log`. JSON is easy to parse but terrible to read in commit history. Git trailers are plain text key-value pairs at the end of commit messages, part of the git spec, parsed natively with `git log --format='%(trailers)'`. Zero dependencies. Humans see a normal commit message. Machines parse the trailers.

**Rejected: JSON blocks in commit messages.** Parseable but unreadable for humans scanning `git log`. Commits should be human-first.

**Rejected: YAML frontmatter.** Not a git convention. Would confuse tools that parse commit messages.

**Rejected: Custom delimited format.** Invents a problem that git trailers already solve.

### Decision: DB checked into main branch (not ephemeral/rebuilt every time)

**Rationale:** Originally designed the DB as ephemeral — rebuilt from scratch on every commit by feeding the full git history to an LLM. This is clean (no drift possible) but expensive. As a project grows to hundreds or thousands of commits, reprocessing everything is wasteful. The DB is now a persistent artifact on main that gets incrementally updated when new commits land. A full rebuild is available as a manual reset when needed.

**Rejected: Ephemeral DB rebuilt from scratch every commit.** Clean but doesn't scale. Reprocessing 500 commits to incorporate 1 new one is wasteful. The full rebuild remains available as a reset path.

**Rejected: DB in .gitignore, generated locally.** Each developer would have a different DB depending on when they last built it. Checking it into main means everyone gets the same knowledge store.

### Decision: DB lives on main only, branches get a snapshot

**Rationale:** SQLite is a binary file. If multiple branches modify it, git can't merge it. Simplest solution: branches never modify the DB. They work off whatever main had when they branched. On merge to main, a hook processes the branch's new commits and updates the DB.

**Rejected: Per-branch DBs merged via SQLite ATTACH.** SQLite supports attaching multiple DBs and querying across them. A branch could maintain its own overlay DB that gets merged at query time. Technically sound but adds complexity. The simple approach (branches use main's snapshot) works 95% of the time. The ATTACH approach is documented as a future option if stale branch memory becomes a real problem.

**Rejected: Conflict resolution on the DB file.** Binary files can't be merged by git. Any strategy involving multiple writers to the same DB introduces conflict risk. Avoiding it entirely is simpler.

### Decision: Single Python script for both MCP server and build agent

**Rationale:** The MCP server and build agent share the same DB schema knowledge. Splitting them into separate files means maintaining schema definitions in two places. One script with multiple entry points (`serve`, `build`, `rebuild`, `export`) keeps everything together. The git hook calls the same script the MCP config points to, just with a different subcommand.

### Decision: File-based retrieval as primary query method (not topic search)

**Rationale:** Coding agents operate on files. They open files, edit files, commit files. Querying "what do you know about this file" is the most natural and precise retrieval method. No keyword ambiguity, no fuzzy matching. The build system already knows which files each commit touched (from the diff), so file associations come for free. Topic-based FTS search exists as a secondary method for broader questions.

**Rejected: Topic search as primary method.** Requires the agent to guess good search terms. "What do you know about webhooks" might miss debt logged against the Subscription model that's related but doesn't mention "webhook." File-based queries don't have this problem.

**Rejected: Vector embeddings / semantic search.** Overkill at project scale. FTS5 is built into SQLite, zero dependencies, and sufficient for searching across dozens to low hundreds of memories. Embeddings add complexity (model dependency, storage, recomputation) for marginal benefit at this scale.

### Decision: Memory linking with typed relationships

**Rationale:** Inspired by the `agent-memory` npm package. Memories are associative, not flat. The webhook architecture memory is related to the middleware hotfix memory. When Opus queries one, the link surfaces the other — even if Opus didn't think to ask about it. This is implemented as a simple join table, not a graph database. Relationship types give the links semantic meaning.

**Rejected: Full graph database (Neo4j, etc.).** Massive overkill. A join table with two foreign keys and a relationship type achieves the same thing within SQLite.

**Rejected: Flat memory list with no relationships.** Loses the associative nature of knowledge. In dry runs, links surfaced critical context (like the middleware warning) that wouldn't have appeared from a direct query alone.

### Decision: Separate confidence and importance axes

**Rationale:** Also inspired by `agent-memory`. A memory can be definitely true but unimportant ("added a favicon"), or uncertain but critical ("something was emergency-fixed in the middleware stack"). Collapsing these into a single score loses information. Opus uses both to decide how to act: high confidence + high importance = trust it; low confidence + high importance = investigate before proceeding.

### Decision: Decay based on access frequency

**Rationale:** From `agent-memory`. Memories that are never queried naturally lose importance over time. This keeps the knowledge store focused on what's actually useful without requiring manual cleanup. Frequently accessed memories stay prominent. Decay is applied during the build step, not at query time.

**Rejected: TTL / expiration-based memory.** Time-based expiration doesn't make sense for project knowledge. A two-year-old architectural decision might still be the most important memory in the store. Decay based on access is a better signal than age.

### Decision: LLM-powered build step (not rule-based parsing)

**Rationale:** The build agent reads commits chronologically and reasons about what they mean. This is fundamentally an LLM task — interpreting diffs, inferring intent from commit messages, recognizing when new information supersedes old, finding connections between memories. A rule-based parser could extract trailer fields from enhanced commits, but it can't derive meaningful memories from raw commits with bare messages and diffs. The LLM handles both tiers of input naturally.

### Idea explored but deferred: Static analysis integration

**Rationale:** Static analysis knows structural things the docs and commits don't capture — dependency graphs, call chains, import maps. A senior developer has this knowledge from reading code over time. We discussed feeding static analysis output into the build step to enrich the knowledge store. Decided to defer for v1 because: the system should prove its value with commits and docs alone first, and static analysis adds dependencies and complexity. If the main gap after v1 testing is "memory doesn't know about code structure," static analysis becomes the v2 priority.

### Idea explored but deferred: Branch-local overlay DBs via SQLite ATTACH

**Rationale:** Described above under branching strategy. The simple approach (branches use main's snapshot) is the v1 path. ATTACH is the escape hatch if needed.

### Idea explored but deferred: Memory docs written to docs folder

**Rationale:** Discussed writing memories as markdown files into the docs folder alongside plan/close docs, creating a human-readable "memory changelog." The DB would be the active working memory while docs would be the permanent journal. Decided this adds complexity for v1 — the DB checked into main already provides persistence, and the JSON export provides human inspectability. Could revisit if there's a need for human-readable memory browsing beyond what export provides.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                    Git Repository                    │
│                                                      │
│  Commits (source of truth for memory)                │
│    ├── Enhanced commits (structured trailers)        │
│    └── Raw commits (message + diff)                  │
│                                                      │
│  Docs folder (supplemental, referenced from commits) │
│    ├── Plan docs                                     │
│    ├── Close docs (planned vs actual, debt)           │
│    └── Memory docs (optional changelog)              │
│                                                      │
│  project_memory.py (single script, checked in)       │
│  project_memory.db (checked into main branch)        │
└─────────────────────────────────────────────────────┘
```

**The single script has four modes:**

```bash
python project_memory.py serve      # MCP server (stdio) — Opus queries this
python project_memory.py build      # Incremental — process new commits, update DB
python project_memory.py rebuild    # Full rebuild from entire git history
python project_memory.py export     # Dump JSON for inspection/debugging
```

- `serve` is what the .agent MCP config points to
- `build` is what the git hook or CI calls on merge to main
- `rebuild` is the manual reset button
- `export` is for debugging the build agent's output

## Data Sources

### Primary: Git Commits

Every commit contributes to memory. Two tiers of input:

**Tier 1 — Raw git (any repo, no setup required):**

- Commit message (whatever quality)
- Diff (files changed, lines added/removed)
- File change patterns
- Timestamps
- Author

**Tier 2 — Enhanced commits (requires commit workflow adoption):**

- Human-readable summary line and body (AI-generated)
- Structured git trailers with explicit fields (see Commit Schema below)
- References to related docs

### Supplemental: Docs Folder

Plan/implement/close documents from the SDLC workflow. When they exist, they add narrative depth — rationale, failed attempts, debt acknowledgments, constraints discovered. Referenced from commits when available. Not required for the system to function.

## Commit Schema (Enhanced Commits)

Enhanced commits use git trailers — plain text, part of the git spec, zero dependencies. Parsed natively with `git log --format='%(trailers)'`.

```
Short summary line for humans reading git log

Longer body in natural language explaining what changed and why.
AI-generated from the agent's working context at commit time.
Captures intent, decisions, tradeoffs — the stuff that evaporates
when the session ends.

Type: feature|fix|refactor|config|debt|docs|test
Rationale: Why this approach was chosen
Rejected: What was considered and not done, and why
Fragile: What's temporary, hardcoded, or likely to break
Related: Slug or reference to related work/commits
Confidence: high|medium|low
Doc-ref: docs/2025-03-09-webhook-refactor.md
```

**Rules:**

- Only `Type` is always present. Everything else is included when relevant.
- The AI generating the commit decides what's worth capturing based on the significance of the change.
- A simple one-line fix might only have `Type: fix`. A major architectural change gets the full set.
- The commit workflow is a customizable `.agent` step, so the format and fields can evolve over time.

## Database Schema

Single SQLite file: `project_memory.db`

### Table: `memories`

```sql
CREATE TABLE memories (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    summary         TEXT NOT NULL,
    type            TEXT NOT NULL,
    confidence      TEXT NOT NULL,       -- high, medium, low
    importance      REAL NOT NULL DEFAULT 0.5,
    source_commits  TEXT NOT NULL DEFAULT '[]',
    source_doc_refs TEXT NOT NULL DEFAULT '[]',
    files           TEXT NOT NULL DEFAULT '[]',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    accessed_at     TEXT NOT NULL,
    access_count    INTEGER NOT NULL DEFAULT 0,
    active          INTEGER NOT NULL DEFAULT 1
);
```

### Table: `memory_links`

```sql
CREATE TABLE memory_links (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    memory_id_a   INTEGER NOT NULL REFERENCES memories(id),
    memory_id_b   INTEGER NOT NULL REFERENCES memories(id),
    relationship  TEXT NOT NULL,
    strength      REAL NOT NULL DEFAULT 0.5,
    created_at    TEXT NOT NULL
);
```

### Table: `build_meta`

```sql
CREATE TABLE build_meta (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    build_type      TEXT NOT NULL,     -- incremental, full
    last_commit     TEXT NOT NULL,     -- hash of last processed commit
    commit_count    INTEGER NOT NULL,
    memory_count    INTEGER NOT NULL,
    built_at        TEXT NOT NULL
);
```

### FTS5 Virtual Table

```sql
CREATE VIRTUAL TABLE memories_fts USING fts5(
    summary,
    content=memories,
    content_rowid=id
);
```

With triggers to keep FTS in sync on INSERT, UPDATE, DELETE.

### Indexes

```sql
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_active ON memories(active);
CREATE INDEX idx_memories_importance ON memories(importance);
CREATE INDEX idx_memories_confidence ON memories(confidence);
```

### File-Based Query Pattern

File paths are stored as a JSON array in the `files` column:

```sql
-- Exact file match
SELECT * FROM memories
WHERE active = 1 AND files LIKE '%"app/Services/PaymentGateway.php"%';

-- Directory match
SELECT * FROM memories
WHERE active = 1 AND files LIKE '%"app/Webhooks/Handlers/%';
```

For the prototype this is sufficient. A normalized `memory_files` junction table can be added later if performance requires it.

## Memory Types

| Type         | What it captures                                                  | Example                                                                     |
| ------------ | ----------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `decision`   | Why something was built/chosen a certain way                      | "Chose handler class pattern over individual routes for webhooks"           |
| `pattern`    | Recurring implementation approaches observed in the code          | "Validation logic lives in Form Request classes"                            |
| `convention` | Established project standards                                     | "Job naming convention is {domain}:{action}"                                |
| `debt`       | Known shortcuts, hardcoded values, things flagged for future work | "Grace period hardcoded to 7 days, should be configurable"                  |
| `bug_fix`    | Bug context — what went wrong and why                             | "Webhook signature verification must be global middleware, not route-level" |
| `context`    | Domain knowledge, business rules, external constraints            | "Stripe rate limits at 100 req/s, production hitting 80+"                   |

## Memory Linking

### Relationship Types

| Relationship       | Meaning                                       | Example                                                 |
| ------------------ | --------------------------------------------- | ------------------------------------------------------- |
| `related_to`       | General association                           | Stripe service ↔ webhook controller                     |
| `supersedes`       | Newer memory replaces older understanding     | Enum status supersedes string status                    |
| `caused_by`        | One thing resulted from another               | Middleware hotfix caused by webhook architecture        |
| `resolved_by`      | A debt or bug was fixed                       | String status debt resolved by enum migration           |
| `implements`       | Concrete implementation of a pattern/decision | Cancellation handler implements webhook handler pattern |
| `convention_group` | Multiple conventions that belong together     | Validation in form requests + auth in middleware        |
| `debt_in`          | Debt associated with a specific area          | Hardcoded grace period is debt in cancellation service  |

### Link Traversal

When a memory is returned from a query, its linked memories can optionally be included. This surfaces context the agent didn't explicitly ask for. Example: querying the Handlers directory returns the webhook architecture memory, which links to the middleware hotfix memory via `caused_by` — surfacing a warning the agent didn't think to search for.

## Decay

Memories that haven't been accessed lose importance over time. Applied during the build step.

- Decay reduces the `importance` score, not confidence
- Decay does not delete memories — just deprioritizes them in query results
- Memories reinforced by new commits (the same files or topics appear again) resist decay
- Start simple: multiply importance by 0.95 for each build cycle where the memory wasn't accessed or reinforced — tune from there

## Retrieval (MCP Server Tools)

### `memory_query_file`

**Primary retrieval method.** Query by file path or directory.

Input: `{ "path": "app/Webhooks/Handlers/" }`

Returns all active memories associated with files under that path, sorted by importance. Includes linked memories optionally.

### `memory_search`

**Secondary method.** Full-text search across memory summaries.

Input: `{ "query": "error handling", "type": "pattern", "min_importance": 0.3 }`

All filters optional. Returns matching memories sorted by FTS rank.

### `memory_get`

Get a specific memory by ID with full detail including all linked memories.

Input: `{ "id": 7, "include_links": true }`

### `memory_stats`

Overview of the knowledge store.

Returns: total count, counts by type, counts by confidence, top files by memory count, average importance, last build timestamp, last processed commit.

### `memory_export`

Dump the full knowledge store as JSON for inspection.

```json
{
  "version": "1.0",
  "format": "project-memory",
  "built_at": "2025-03-09T10:30:00Z",
  "build_type": "incremental",
  "last_commit": "def5678",
  "stats": {
    "total_memories": 42,
    "by_type": {},
    "by_confidence": {},
    "avg_importance": 0.65
  },
  "memories": [...],
  "links": [...]
}
```

## Build System

### Incremental Build (Normal Path)

Triggered on merge to main via git hook or CI.

1. Read `build_meta` to find the last processed commit hash
2. Extract new commits since that hash: `git log <last_hash>..HEAD -p --reverse`
3. Parse trailers from enhanced commits; pass raw commits as-is
4. If any commits have `Doc-ref` trailers, read those doc files
5. Feed the new commits + existing memories to the LLM
6. LLM produces: new memories, updates to existing memories, new links, confidence/importance adjustments
7. Apply decay to memories not accessed or reinforced in this cycle
8. Write changes to DB
9. Update `build_meta` with new last commit hash
10. Commit the updated DB to main

### Full Rebuild (Reset Path)

Triggered manually: `python project_memory.py rebuild`

1. Drop all existing memories, links, and build_meta
2. Extract full git history: `git log -p --reverse`
3. Read all docs in the docs folder
4. Feed everything chronologically to the LLM
5. LLM produces the complete knowledge store from scratch
6. Write to DB

### Build Agent LLM Prompt (Guidance)

The build agent prompt should instruct the LLM to:

- Read commits in chronological order
- For each commit, consider: what changed (diff), why (message/trailers), which files, and how it relates to existing memories
- Only create memories where there is clear evidence — never infer beyond what the commit shows
- Mark confidence based on evidence quality:
  - `high` = explicit in trailers or detailed commit message
  - `medium` = inferred from clear diff patterns across multiple commits
  - `low` = inferred from a single ambiguous diff or bare message
- Score importance based on how much the memory would affect future development decisions
- When a new commit contradicts or resolves an existing memory, update accordingly — don't keep both conflicting versions active
- Create links between related memories with appropriate relationship types
- For raw commits with bare messages ("hotfix", "fix stuff", "wip"), derive what you can from the diff and mark confidence as low
- When a commit references a doc (`Doc-ref` trailer), read the doc for additional context and potentially upgrade confidence of memories derived from that commit
- Never produce a memory about something the commits don't provide evidence for — silence is better than fabrication
- Consider file co-change patterns — files that always change together in commits are likely coupled

### Git Commands for Extracting History

```bash
# New commits since last build (incremental)
git log <last_hash>..HEAD -p --reverse --format='commit %H%nAuthor: %an%nDate: %ai%n%n%B%n%(trailers)%n---'

# Full history (rebuild)
git log -p --reverse --format='commit %H%nAuthor: %an%nDate: %ai%n%n%B%n%(trailers)%n---'

# Just messages and file lists (lighter weight, for initial prototype testing)
git log --stat --reverse --format='commit %H%nAuthor: %an%nDate: %ai%n%n%B%n%(trailers)%n---'

# Extract trailers only from a commit
git log -1 --format='%(trailers)' <hash>
```

## Repository Layout

```
project-root/
├── .agent/                          # Existing workflow rules
│   └── workflows/
│       └── commit.md                # Enhanced commit workflow step
├── docs/                            # Existing SDLC docs (supplemental)
│   ├── 2025-01-15-auth-setup.md
│   ├── 2025-02-10-billing-refactor.md
│   └── finished/
├── project_memory.py                # Single script: MCP server + build agent
├── project_memory.db                # Knowledge store (checked into main)
├── .gitattributes                   # Marks .db as binary
└── src/                             # Project source code
```

### .gitattributes

```
project_memory.db binary
```

### MCP Configuration (for .agent or claude_desktop_config.json)

```json
{
  "mcpServers": {
    "project-memory": {
      "command": "python",
      "args": ["project_memory.py", "serve"]
    }
  }
}
```

### Git Hook (post-merge or CI step)

```bash
#!/bin/bash
python project_memory.py build
git add project_memory.db
git commit -m "chore: update project memory store"
```

## Branching & Merge Strategy

- **The DB lives on main only.** It is checked into source control on the main branch.
- **Feature branches get a snapshot.** When a developer branches off main, they get the DB as it was at that point. Memory is slightly behind but still useful.
- **Branches don't modify the DB.** No binary merge conflicts.
- **On merge to main**, a post-merge hook or CI step runs the build agent. It processes the new commits from the merged branch against the existing DB and commits the updated DB to main.
- **If the DB conflicts** (two PRs merge close together), take main's version and reprocess. No data loss — commits are the source of truth and the DB can always be rebuilt.

## Integration with .agent Workflow

The `.agent` rules instruct Opus to:

- **Before editing any file:** query `memory_query_file` for that file/directory
- **Before starting a new feature area:** query `memory_search` for the relevant domain
- **When memory returns results:** treat them as hints, verify against actual code, trust high-confidence more than low
- **At commit time:** generate a structured commit message using the enhanced commit schema with trailers, capturing the AI's working context before it evaporates
- **When discovering something worth noting:** if it belongs in a plan/close doc, write it there; the build system will pick it up from the commit

## Dependencies

- Python 3.10+
- `mcp` Python SDK (`pip install mcp`)
- `sqlite3` (stdlib)
- `json` (stdlib)
- `subprocess` (stdlib, for git commands)
- Access to an LLM API for the build step (Anthropic API for Claude, or configurable)

## What This System Does NOT Do

- **It is not a vector database.** FTS5 is sufficient at project scale.
- **It does not run as a daemon or background process.** Build runs on merge. MCP serves on demand.
- **It does not require any specific workflow.** It works on any git repo. Enhanced commits and docs make it better but aren't required.
- **It does not replace reading the code.** It reduces discovery time. The code is always the authority.
- **It does not guess.** If it doesn't know, it says nothing.
- **It does not capture conversational context directly.** Chat between developer and AI is captured indirectly through enriched commit messages written at commit time.

## MVP Prototype Plan

### Phase 1: Validate the concept

1. Pick a real repo with history
2. Run `git log -30 --stat --reverse` to extract recent history
3. Feed it to an LLM with the build agent prompt guidance from this spec
4. Evaluate: are the memories useful? Would they have prevented recent bad iterations?
5. Simulate queries: ask about specific files and topics, assess the answers

### Phase 2: Build the read path

1. Create the SQLite schema
2. Manually populate it with the memories from Phase 1
3. Implement the MCP server (`serve` mode) with the query tools
4. Wire it into a real project's .agent config
5. Use it during actual development and evaluate

### Phase 3: Build the write path

1. Implement the `build` command — git log parsing, LLM call, DB updates
2. Implement the `rebuild` command
3. Set up the git hook
4. Run on real commits and evaluate the incremental build quality

### Phase 4: Enhanced commits

1. Create the `.agent/workflows/commit.md` workflow step
2. Start generating structured commits with trailers
3. Evaluate whether trailers meaningfully improve memory quality over raw commits

## Open Questions

- Exact decay rate and formula — start with 0.95 per build cycle, tune from there
- Build agent LLM prompt optimization — the guidance above is directional, the exact prompt needs iteration
- Whether static analysis should feed into the build step (deferred to v2)
- Optimal commit trailer fields beyond the initial set — discover through use
- Size management for very large/old repos — when does full rebuild become impractical, what's the chunking strategy
- Whether branch-local overlay DBs (via SQLite ATTACH) are needed in practice
- How to handle the LLM API dependency in the build step — should it be configurable (Claude, GPT, local model)?
- Whether the build step should also read the current state of referenced files (not just the diff) for richer contexts
