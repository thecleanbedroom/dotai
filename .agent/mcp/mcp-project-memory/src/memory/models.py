"""Data classes and constants for the project memory system."""

import uuid
from dataclasses import dataclass, field

MEMORY_TYPES = frozenset({
    "decision", "pattern", "convention", "debt", "bug_fix", "context",
    "refactor", "fix", "feature",
})


RELATIONSHIP_TYPES = frozenset({
    "related_to", "supersedes", "caused_by", "resolved_by",
    "implements", "convention_group", "debt_in",
})


def _new_uuid() -> str:
    return str(uuid.uuid4())


@dataclass
class Memory:
    """A single memory entry.

    The `id` is a UUID string — unique across developers, no coordination
    needed.  JSON files on disk are the canonical store; the SQLite DB is
    a disposable runtime cache.
    """
    id: str = field(default_factory=_new_uuid)
    summary: str = ""
    type: str = "context"
    confidence: int = 0
    importance: int = 50
    source_commits: list[str] = field(default_factory=list)
    file_paths: list[str] = field(default_factory=list)
    tags: list[str] = field(default_factory=list)
    links: list[dict] = field(default_factory=list)
    created_at: str = ""
    active: bool = True

    # ── DB-only ephemeral fields (not persisted to JSON) ──
    accessed_at: str = ""
    access_count: int = 0

    # ── Serialization ──

    def to_json_dict(self) -> dict:
        """Return the canonical JSON representation (for disk storage)."""
        return {
            "id": self.id,
            "type": self.type,
            "summary": self.summary,
            "confidence": self.confidence,
            "importance": self.importance,
            "source_commits": self.source_commits,
            "file_paths": self.file_paths,
            "tags": self.tags,
            "links": self.links,
            "created_at": self.created_at,
            "active": self.active,
        }

    @classmethod
    def from_json_dict(cls, data: dict) -> "Memory":
        """Construct a Memory from a JSON dict (as read from disk)."""
        return cls(
            id=data["id"],
            type=data.get("type", "context"),
            summary=data.get("summary", ""),
            confidence=data.get("confidence", 0),
            importance=data.get("importance", 50),
            source_commits=data.get("source_commits", []),
            file_paths=data.get("file_paths", []),
            tags=data.get("tags", []),
            links=data.get("links", []),
            created_at=data.get("created_at", ""),
            active=data.get("active", True),
        )

    def to_dict(self) -> dict:
        """Full dict including DB-only fields (for MCP responses)."""
        d = self.to_json_dict()
        d["accessed_at"] = self.accessed_at
        d["access_count"] = self.access_count
        return d


@dataclass
class MemoryLink:
    """A typed relationship between two memories (UUID-based)."""
    id: int | None = None
    memory_id_a: str = ""
    memory_id_b: str = ""
    relationship: str = "related_to"
    strength: float = 0.5
    created_at: str = ""


@dataclass
class BuildMetaEntry:
    """Metadata about a build run."""
    id: int | None = None
    build_type: str = "incremental"
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
