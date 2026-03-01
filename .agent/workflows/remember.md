---
description: "Persist knowledge to Antigravity's KI system so it surfaces automatically in future conversations. Supports conversation scanning, direct input, and file consumption."
---

# /remember — Persist Knowledge

Save insights, decisions, and tribal knowledge so it surfaces automatically in future work. Knowledge is routed to the right storage based on scope:

| Scope       | Storage                                 | When it surfaces                              |
| ----------- | --------------------------------------- | --------------------------------------------- |
| **Global**  | `~/.gemini/antigravity/knowledge/` (KI) | All future conversations, any project         |
| **Project** | `.agent/rules/` (rule file)             | Only when working in this project's workspace |

**CRITICAL: Never persist without explicit user approval.**

## Modes

| Invocation          | Mode       | What happens                                                                |
| ------------------- | ---------- | --------------------------------------------------------------------------- |
| `/remember`         | **Scan**   | Analyze the current conversation for memorable insights, present candidates |
| `/remember <text>`  | **Direct** | Craft a memory from the user's statement                                    |
| `/remember @[file]` | **File**   | Consume referenced file(s), summarize learnings                             |

All three modes converge at **Step 2: Determine Scope**.

---

## Steps

### 1. Extract knowledge

#### Scan mode (no input)

Review the current conversation for insights worth persisting. Look for:

- Architectural decisions or trade-offs
- Debugging discoveries (root cause + fix)
- Environment/infrastructure facts ("we use Docker", "tests run inside the container")
- Gotchas, edge cases, or non-obvious behaviors
- Workflow preferences or conventions
- Integration details (API quirks, config nuances)

Build a **numbered list of candidates**, each with a one-line summary. Present the list and ask the user to pick which ones to save (comma-separated numbers or "all"). Wait for their selection before proceeding.

#### Direct mode (text input)

The user provided the knowledge directly (e.g., `/remember we use docker and testing needs to be ran inside docker`).

Parse the input and expand it into a well-structured insight. Add context and implications that would make this useful to a future agent. For example, "we use Docker" becomes guidance about which commands to run inside vs. outside the container, how to invoke the test suite, etc.

#### File mode (file input)

Read the referenced file(s). Extract the key learnings, patterns, decisions, or facts. Summarize what a future agent should know from this content. If the file is large, focus on the most actionable insights rather than reproducing everything.

---

### 2. Determine scope

For each piece of knowledge, decide whether it is **global** or **project-specific**:

**Try global first.** If the insight can be framed in a way that's useful across projects, write it as a global KI. Example: "When a project uses Docker, always run tests inside the container" is globally useful advice.

**Declare project-specific** when the knowledge is inherently bound to this project's stack, config, or codebase. Examples:

- "This project uses Docker" → project (other projects may not)
- "Tests must run via `docker exec app php artisan test`" → project
- "The Shopify webhook signature uses HMAC-SHA256" → global (applies to any Shopify integration)
- "Eric prefers conventional commit messages" → global (personal preference across all work)

**Decision rules:**

| Signal                                                     | Scope                         |
| ---------------------------------------------------------- | ----------------------------- |
| References this project's specific tools, stack, or config | **Project**                   |
| References specific file paths in this repo                | **Project**                   |
| A general pattern, best practice, or API behavior          | **Global**                    |
| A personal preference or cross-cutting convention          | **Global**                    |
| Could help in a different project with similar tech        | **Global** — frame it broadly |

Include the determined scope in the preview (Step 3). The user can override.

---

### 3. Present for approval

For each proposed memory, present the user with a clear preview:

```
## Proposed Memory: <topic_slug>

**Scope**: 🌐 Global (KI) / 📁 Project (.agent/rules/)
**Title**: <Human-readable title>
**Summary**: <2-3 sentence summary>

**Content preview**:
<the artifact or rule content>
```

The user must explicitly approve before proceeding. They may:

- **Approve** — proceed to persist
- **Edit** — request changes to the title, summary, slug, scope, or content
- **Skip** — discard this candidate (in scan mode, move to the next)

Loop on edits until approved or skipped.

---

### 4. Check for duplicates

#### For global scope (KIs)

```bash
ls ~/.gemini/antigravity/knowledge/
```

Read `metadata.json` from any KI whose slug or title looks similar. If a duplicate or near-duplicate exists:

- Tell the user: "A KI already exists at `<slug>` with title '<title>'. Would you like to **update** the existing KI or **create a new one**?"
- If updating, merge the new content into the existing artifact and update the metadata summary and timestamps.

#### For project scope (rules)

```bash
ls .agent/rules/
```

Read existing rule files to check if the knowledge is already captured. If so, offer to **append** to or **update** the existing rule file.

---

### 5. Persist

#### Global scope → KI

Write three files to `~/.gemini/antigravity/knowledge/<topic_slug>/`:

**`metadata.json`**

```json
{
  "title": "<Human-readable title>",
  "summary": "<2-3 sentence summary for system prompt injection>",
  "references": [
    {
      "type": "conversation_id",
      "value": "<current conversation UUID>"
    }
  ],
  "artifacts": ["artifacts/<document_name>.md"]
}
```

- The `summary` field is **critical** — it's what gets injected into the agent's pre-flight prompt and determines whether the KI is surfaced. Make it specific and keyword-rich.
- Add `file` references if the knowledge came from specific source files.
- If updating an existing KI, **append** the new conversation reference rather than replacing existing ones.

**`timestamps.json`**

```json
{
  "created": "<ISO 8601 with timezone>",
  "modified": "<ISO 8601 with timezone>",
  "accessed": "<ISO 8601 with timezone>"
}
```

Use the current local time. When updating an existing KI, preserve the original `created` timestamp and update `modified` and `accessed`.

**`artifacts/<document_name>.md`**

Structured knowledge artifact:

```markdown
# <Title>

## Context

What was happening, why this matters.

## Insight

The actual rule, lesson, fact, or decision to remember.

## Details

Optional deeper technical information, code snippets, examples.
```

#### Project scope → Rule

Append to an existing rule file or create a new one in `.agent/rules/`:

- **Match by category**: if the knowledge fits an existing file (e.g., Docker knowledge → `platform-docker.md`, Laravel specifics → `platform-laravel.md`), append to it.
- **New file**: if no existing file fits, create a new one following the naming convention (`platform-*.md` for platform/tool rules, `core-*.md` for workflow rules).
- Use the same directive style as existing rule files — concise, imperative statements.

---

### 6. Confirm

Report to the user:

| Memory    | Scope      | Location                                  |
| --------- | ---------- | ----------------------------------------- |
| `<title>` | 🌐 Global  | `~/.gemini/antigravity/knowledge/<slug>/` |
| `<title>` | 📁 Project | `.agent/rules/<file>.md`                  |

- For global KIs: "This will automatically surface in future conversations when relevant topics are discussed."
- For project rules: "This will apply whenever working in this project's workspace."

If multiple memories were created (scan mode), summarize all of them in the table.
