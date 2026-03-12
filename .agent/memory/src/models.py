"""Data classes and constants for the project memory system."""

from dataclasses import dataclass, field, asdict
from typing import Optional


MEMORY_TYPES = frozenset({
    "decision", "pattern", "convention", "debt", "bug_fix", "context",
    "refactor", "fix", "feature",
})

CONFIDENCE_LEVELS = frozenset({"high", "medium", "low"})

RELATIONSHIP_TYPES = frozenset({
    "related_to", "supersedes", "caused_by", "resolved_by",
    "implements", "convention_group", "debt_in",
})


@dataclass
class Memory:
    """A single memory entry."""
    id: Optional[int] = None
    summary: str = ""
    type: str = "context"
    confidence: str = "medium"
    importance: float = 0.5
    source_commits: list[str] = field(default_factory=list)
    source_doc_refs: list[str] = field(default_factory=list)
    files: list[str] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)
    created_at: str = ""
    updated_at: str = ""
    accessed_at: str = ""
    access_count: int = 0
    active: bool = True

    def to_dict(self) -> dict:
        d = asdict(self)
        d["active"] = int(d["active"])
        return d


@dataclass
class MemoryLink:
    """A typed relationship between two memories."""
    id: Optional[int] = None
    memory_id_a: int = 0
    memory_id_b: int = 0
    relationship: str = "related_to"
    strength: float = 0.5
    created_at: str = ""


@dataclass
class BuildMetaEntry:
    """Metadata about a build run."""
    id: Optional[int] = None
    build_type: str = "incremental"
    last_commit: str = ""
    commit_count: int = 0
    memory_count: int = 0
    built_at: str = ""


@dataclass
class ParsedCommit:
    """A parsed git commit with optional trailers."""
    hash: str = ""
    author: str = ""
    date: str = ""
    message: str = ""
    body: str = ""
    diff: str = ""
    files: list[str] = field(default_factory=list)
    trailers: dict[str, str] = field(default_factory=dict)
