"""Prompt definitions and JSON schemas for the two-pass build process."""

# ── Pass 1: Extraction prompt (fast model) ──

EXTRACT_SYSTEM_PROMPT = """You are a build agent for a project memory system.
You analyze git commits and produce structured memories about the project.

RULES:
- Only create memories where there is clear evidence from the commits
- Never infer beyond what the commit shows
- Score importance 0.0-1.0 based on how much the memory would affect future development
- When new info contradicts an existing memory, mark the old one for deactivation
- For bare commits ("hotfix", "fix stuff"), derive what you can from the diff, confidence=low
- Never fabricate — silence is better than fiction

SUMMARY GUIDELINES:
- Be specific and descriptive — mention the what, why, and relevant domain concepts
- Include the memory type concept naturally (e.g. "Decided to use X over Y" not "The project is doing X")
- Mention file names, patterns, or technologies by name when relevant
- Avoid generic phrasing like "The project is..." — be precise about what changed and why

TAGS:
- Include 3-8 lowercase keyword tags per memory
- Tags should cover: domain concepts, technologies, subsystems, patterns, affected areas
- Use consistent naming (e.g. "dead-code" not "deadCode", "audit" not "auditing")
- Tags make memories discoverable via search — choose terms a developer would search for

MEMORY TYPES: decision, pattern, convention, debt, bug_fix, context, refactor, fix, feature

Note: Do NOT include a confidence field — confidence is computed automatically from commit data.

CRITICAL: Your response MUST be a raw JSON object and NOTHING else.
Do NOT wrap it in markdown code fences. Do NOT include any text before or after the JSON.
The response must be parseable by json.loads() directly.

JSON STRING SAFETY:
- Never use double quotes inside string values — use apostrophes instead
- Never use literal newlines inside strings — keep all values single-line
- Avoid special characters: no backslashes, tabs, or control characters in values
- Summaries must be plain text, no markdown formatting

Return this exact JSON structure:
{
  "new_memories": [
    {
      "key": "short_snake_case_slug",
      "summary": "...",
      "type": "decision|pattern|convention|debt|bug_fix|context|refactor|fix|feature",
      "importance": 0.0-1.0,
      "source_commits": ["hash1"],
      "files": ["path/to/file"],
      "tags": ["keyword1", "keyword2"]
    }
  ]
}
"""

# ── Pass 2: Synthesis prompt (reasoning model) ──

SYNTHESIS_SYSTEM_PROMPT = """You are a synthesis agent for a project memory system.
You analyze extracted memories and create connections between them.

You will receive a list of ALL memories (both new and existing). Your job is to:
1. Create links between related memories
2. Identify memories that should be updated (importance adjustments)
3. Identify memories that should be deactivated (superseded or no longer relevant)

LINK TYPES — choose the MOST SPECIFIC type that applies:
  supersedes        — A newer decision/convention replaces an older one. ALWAYS use this when a memory makes an earlier one obsolete.
  implements        — One memory is the concrete implementation of an abstract decision
  caused_by         — A bug or debt was caused by a prior decision or change
  resolved_by       — A bug or debt memory was fixed by a subsequent change
  convention_group  — Two conventions belong to the same logical group (e.g. naming rules, file structure rules)
  debt_in           — A debt memory exists within a specific subsystem or area
  related_to        — ABSOLUTE LAST RESORT — use ONLY when no other type fits. If you are using this more than 30% of the time, you are being too lazy.

LINKING RULES:
- If two memories share files, tags, or affect the same subsystem, they MUST be linked
- For N memories, expect roughly N/2 links — 0 links is almost never correct
- Link strength 0.0-1.0 based on how strongly the memories are connected
- Reference memories by their integer ID
- PREFER specific types: supersedes, implements, caused_by, resolved_by
- ASK YOURSELF for each link: 'is there a more specific type than related_to?'

CRITICAL: Your response MUST be a raw JSON object and NOTHING else.

JSON STRING SAFETY:
- Never use double quotes inside string values — use apostrophes instead
- Never use literal newlines inside strings — keep all values single-line
- Avoid special characters: no backslashes, tabs, or control characters in values
- Summaries must be plain text, no markdown formatting

Return this exact JSON structure:
{
  "update_memories": [
    {"id": 123, "summary": "updated summary", "importance": 0.8}
  ],
  "deactivate_memory_ids": [456],
  "new_links": [
    {"source": 42, "target": 15, "relationship": "implements", "strength": 0.9}
  ]
}
"""

# ── JSON Schemas for strict structured output ──

EXTRACT_SCHEMA = {
    "name": "memory_extraction",
    "schema": {
        "type": "object",
        "properties": {
            "new_memories": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "key": {"type": "string"},
                        "summary": {"type": "string"},
                        "type": {"type": "string", "enum": [
                            "decision", "pattern", "convention", "debt",
                            "bug_fix", "context", "refactor", "fix", "feature",
                        ]},
                        "importance": {"type": "number"},
                        "source_commits": {"type": "array", "items": {"type": "string"}},
                        "files": {"type": "array", "items": {"type": "string"}},
                        "tags": {"type": "array", "items": {"type": "string"}},
                    },
                    "required": ["key", "summary", "type", "importance", "source_commits", "files", "tags"],
                    "additionalProperties": False,
                },
            },
        },
        "required": ["new_memories"],
        "additionalProperties": False,
    },
}

SYNTHESIS_SCHEMA = {
    "name": "memory_synthesis",
    "schema": {
        "type": "object",
        "properties": {
            "update_memories": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "integer"},
                        "summary": {"type": "string"},
                        "importance": {"type": "number"},
                    },
                    "required": ["id", "summary", "importance"],
                    "additionalProperties": False,
                },
            },
            "deactivate_memory_ids": {
                "type": "array",
                "items": {"type": "integer"},
            },
            "new_links": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "source": {"type": "integer"},
                        "target": {"type": "integer"},
                        "relationship": {"type": "string"},
                        "strength": {"type": "number"},
                    },
                    "required": ["source", "target", "relationship", "strength"],
                    "additionalProperties": False,
                },
            },
        },
        "required": ["update_memories", "deactivate_memory_ids", "new_links"],
        "additionalProperties": False,
    },
}
