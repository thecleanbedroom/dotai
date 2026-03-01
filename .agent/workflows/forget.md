---
description: "Review and remove outdated Knowledge Items from Antigravity's KI system. Identifies KIs superseded by newer decisions, changed architecture, or evolved conventions."
---

# /forget — Prune Outdated Knowledge

Review existing knowledge (both global KIs and project rules) and remove or update entries that are no longer accurate. Prevents stale context from misleading the agent in future conversations.

**CRITICAL: Never delete or modify knowledge without explicit user approval.**

## What gets audited

| Scope          | Location                           | Contains                    |
| -------------- | ---------------------------------- | --------------------------- |
| 🌐 **Global**  | `~/.gemini/antigravity/knowledge/` | Cross-project KIs           |
| 📁 **Project** | `.agent/rules/`                    | Project-specific directives |

## Modes

| Invocation        | Mode         | What happens                                                                 |
| ----------------- | ------------ | ---------------------------------------------------------------------------- |
| `/forget`         | **Audit**    | Review all KIs + project rules against current conversation, flag candidates |
| `/forget <topic>` | **Targeted** | Search both KIs and rules matching the topic, present for review             |
| `/forget @[file]` | **Diff**     | Compare file contents against KIs and rules; flag contradictions             |

All three modes converge at **Step 2: Present for Approval**.

---

## Steps

### 1. Identify candidates

#### Audit mode (no input)

Scan both sources:

**Global KIs:**

```bash
ls ~/.gemini/antigravity/knowledge/
```

For each KI directory (skip `knowledge.lock`), read `metadata.json` and the artifact files.

**Project rules:**

```bash
ls .agent/rules/
```

Read each rule file and review its directives.

Cross-reference against the current conversation to identify entries that are:

- **Superseded** — a decision or architecture has changed (e.g., "we use SQLite" when we've since migrated to PostgreSQL)
- **Stale** — references files, patterns, or systems that no longer exist
- **Redundant** — duplicated across KIs and rules, or covered by both
- **Incorrect** — contains factual errors discovered during this conversation
- **Misscoped** — a global KI that should be a project rule (or vice versa)

Build a **numbered list of candidates**, each with:

1. The entry name, scope (🌐 or 📁), and location
2. A one-line reason it may be outdated
3. A recommendation: **delete**, **update**, **move** (rescope), or **keep**

Present the list and wait for the user to select which ones to act on.

#### Targeted mode (topic input)

Search existing KI slugs/titles/summaries AND rule file contents for matches against the user's topic. Present matches with their current content and ask the user what's changed.

#### Diff mode (file input)

Read the referenced file(s) and all existing KI artifacts + rule files. Identify any entries that contradict or are superseded by the file contents. Present the conflicts.

---

### 2. Present for approval

For each candidate, show:

```
## <Scope Emoji> <name>

**Scope**: 🌐 Global KI / 📁 Project Rule
**Location**: <path>
**Reason flagged**: <why this may be outdated>
**Recommendation**: Delete / Update / Move / Keep

**Current content** (collapsed if long):
<content>
```

If recommending **update**, also show the proposed replacement content.
If recommending **move** (rescope), explain where it should go and why.

The user must choose for each candidate:

- **Delete** — remove the KI directory or rule content
- **Update** — modify with corrected content
- **Move** — rescope (global→project or project→global) via `/remember`
- **Keep** — leave unchanged (false positive)
- **Skip** — decide later

---

### 3. Execute

#### For global KI deletions

```bash
rm -rf ~/.gemini/antigravity/knowledge/<topic_slug>/
```

#### For global KI updates

- Update the artifact markdown with corrected content
- Update `metadata.json` summary if it's now inaccurate
- Append the current conversation to `references`
- Update `timestamps.json`: preserve `created`, set new `modified` and `accessed`

#### For project rule deletions

- Remove the specific directives from the rule file (not the whole file unless it's entirely obsolete)
- If the entire file is obsolete, delete it

#### For project rule updates

- Edit the rule file with corrected directives
- Maintain the existing file's style and formatting

#### For moves (rescope)

- Delete from the current location
- Use the `/remember` workflow's persist step to write to the new scope

---

### 4. Confirm

Report to the user:

| Entry    | Scope | Action  | Result       |
| -------- | ----- | ------- | ------------ |
| `<name>` | 🌐    | Deleted | ✅ Removed   |
| `<name>` | 📁    | Updated | ✅ Modified  |
| `<name>` | 🌐→📁 | Moved   | ✅ Rescoped  |
| `<name>` | 📁    | Kept    | ⏭️ No change |

- Deleted entries will no longer surface in future conversations.
- Updated entries will reflect the corrected information.
- Moved entries will now surface in the correct scope.
