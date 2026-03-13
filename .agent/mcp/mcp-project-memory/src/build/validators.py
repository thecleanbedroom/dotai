"""Validators for structured LLM output.

Hard-fail on missing or invalid fields rather than silently patching.
Applied to both extraction and synthesis LLM responses.
"""

from src.memory.models import MEMORY_TYPES, RELATIONSHIP_TYPES
from src.config.internal import InternalSettings


class LLMOutputError(ValueError):
    """Raised when LLM returns structurally invalid output."""


def validate_extraction(result: dict) -> None:
    """Validate extraction LLM response structure.

    Expects: {"new_memories": [...], "updated_memories": [...]}
    Each memory must have: summary, type, source_commits, created_at.

    Raises LLMOutputError on any violation.
    """
    if not isinstance(result, dict):
        raise LLMOutputError(f"Expected dict, got {type(result).__name__}")

    new_memories = result.get("new_memories")
    if new_memories is None:
        raise LLMOutputError("Missing 'new_memories' key in extraction result")
    if not isinstance(new_memories, list):
        raise LLMOutputError(f"'new_memories' must be a list, got {type(new_memories).__name__}")

    for i, mem in enumerate(new_memories):
        _validate_memory_dict(mem, label=f"new_memories[{i}]")


def validate_triage(result: dict) -> None:
    """Validate Phase 1 (triage) LLM response structure.

    Expects: accepted_ids, rejected_ids, deactivate_existing (lists of str).

    Raises LLMOutputError on any violation.
    """
    if not isinstance(result, dict):
        raise LLMOutputError(f"Expected dict, got {type(result).__name__}")

    if "error" in result:
        return

    required_lists = ["accepted_ids", "rejected_ids", "deactivate_existing"]
    for key in required_lists:
        val = result.get(key)
        if val is not None and not isinstance(val, list):
            raise LLMOutputError(f"'{key}' must be a list, got {type(val).__name__}")


def validate_linking(result: dict) -> None:
    """Validate Phase 2 (linking) LLM response structure.

    Expects: update_existing (list of dicts), new_links (list of link dicts).

    Raises LLMOutputError on any violation.
    """
    if not isinstance(result, dict):
        raise LLMOutputError(f"Expected dict, got {type(result).__name__}")

    if "error" in result:
        return

    updates = result.get("update_existing", [])
    if not isinstance(updates, list):
        raise LLMOutputError(
            f"'update_existing' must be a list, got {type(updates).__name__}"
        )
    for i, upd in enumerate(updates):
        if not isinstance(upd, dict):
            raise LLMOutputError(f"update_existing[{i}] must be a dict")
        if "id" not in upd:
            raise LLMOutputError(f"update_existing[{i}] missing 'id'")

    links = result.get("new_links", [])
    if not isinstance(links, list):
        raise LLMOutputError(
            f"'new_links' must be a list, got {type(links).__name__}"
        )
    for i, link in enumerate(links):
        _validate_link_dict(link, label=f"new_links[{i}]")


def _validate_memory_dict(mem: dict, *, label: str) -> None:
    """Validate a single memory dict from LLM output."""
    if not isinstance(mem, dict):
        raise LLMOutputError(f"{label}: expected dict, got {type(mem).__name__}")

    # Required non-empty strings
    for field in ("summary", "created_at"):
        val = mem.get(field)
        if not val or not isinstance(val, str) or not val.strip():
            raise LLMOutputError(f"{label}: '{field}' is required and must be non-empty")

    # Type must be valid
    mem_type = mem.get("type", "")
    if mem_type and mem_type not in MEMORY_TYPES:
        raise LLMOutputError(
            f"{label}: invalid type '{mem_type}', must be one of {sorted(MEMORY_TYPES)}"
        )

    # source_commits must be non-empty list of strings
    commits = mem.get("source_commits")
    if not commits or not isinstance(commits, list):
        raise LLMOutputError(f"{label}: 'source_commits' must be a non-empty list")
    for j, h in enumerate(commits):
        min_len = InternalSettings.validation_min_commit_hash_length()
        if not isinstance(h, str) or len(h) < min_len:
            raise LLMOutputError(
                f"{label}: source_commits[{j}] must be a commit hash string"
            )

    # importance must be int 0-100
    importance = mem.get("importance")
    if importance is not None:
        max_imp = InternalSettings.validation_max_importance()
        if not isinstance(importance, int) or importance < 0 or importance > max_imp:
            raise LLMOutputError(
                f"{label}: 'importance' must be int 0-{max_imp}, got {importance!r}"
            )


def _validate_link_dict(link: dict, *, label: str) -> None:
    """Validate a single link dict from synthesis output."""
    if not isinstance(link, dict):
        raise LLMOutputError(f"{label}: expected dict, got {type(link).__name__}")
    for field in ("source", "target"):
        if not link.get(field):
            raise LLMOutputError(f"{label}: '{field}' is required")
    rel = link.get("relationship", "")
    if rel and rel not in RELATIONSHIP_TYPES:
        raise LLMOutputError(
            f"{label}: invalid relationship '{rel}', "
            f"must be one of {sorted(RELATIONSHIP_TYPES)}"
        )
