"""CRUD stores for memories, links, and build metadata.

All stores operate on the SQLite runtime cache.  The canonical data
lives in JSON files on disk — the DB is rebuilt from those files.

Stores use Database's public API (execute/query/query_one) and never
access the connection directly.  Commit is handled by Database.
"""

import json
import re
import sqlite3
from datetime import UTC, datetime

from src.db import Database
from src.memory.models import BuildMetaEntry, Memory, MemoryLink


class MemoryStore:
    """CRUD for the memories table, file-based queries, FTS5 search."""

    def __init__(self, db: Database):
        self._db = db

    @staticmethod
    def _now() -> str:
        return datetime.now(UTC).isoformat()

    def _row_to_memory(self, row: sqlite3.Row) -> Memory:
        return Memory(
            id=row["id"],
            summary=row["summary"],
            type=row["type"],
            confidence=row["confidence"],
            importance=row["importance"],
            source_commits=json.loads(row["source_commits"]),
            file_paths=json.loads(row["file_paths"]),
            tags=json.loads(row["tags"]),
            created_at=row["created_at"],
            accessed_at=row["accessed_at"],
            access_count=row["access_count"],
            active=bool(row["active"]),
        )

    def create(self, memory: Memory) -> Memory:
        """Insert a new memory and return it."""
        now = self._now()
        memory.accessed_at = memory.accessed_at or now

        self._db.execute(
            """INSERT INTO memories
               (id, summary, type, confidence, importance, source_commits,
                file_paths, tags, created_at, accessed_at,
                access_count, active)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            (
                memory.id, memory.summary, memory.type,
                memory.confidence, memory.importance,
                json.dumps(memory.source_commits),
                json.dumps(memory.file_paths), json.dumps(memory.tags),
                memory.created_at,
                memory.accessed_at, memory.access_count, int(memory.active),
            ),
        )
        return memory

    def get(self, memory_id: str) -> Memory | None:
        """Get a single memory by UUID."""
        row = self._db.query_one(
            "SELECT * FROM memories WHERE id = ?", (memory_id,)
        )
        if row is None:
            return None
        return self._row_to_memory(row)

    def get_many(self, memory_ids: list[str]) -> list[Memory]:
        """Get multiple memories by UUID in a single query."""
        if not memory_ids:
            return []
        placeholders = ",".join("?" for _ in memory_ids)
        rows = self._db.query(
            f"SELECT * FROM memories WHERE id IN ({placeholders})",
            tuple(memory_ids),
        )
        return [self._row_to_memory(r) for r in rows]

    def update(self, memory: Memory) -> None:
        """Update an existing memory."""
        self._db.execute(
            """UPDATE memories SET
               summary=?, type=?, confidence=?, importance=?, source_commits=?,
               file_paths=?, tags=?, accessed_at=?,
               access_count=?, active=?
               WHERE id=?""",
            (
                memory.summary, memory.type, memory.confidence, memory.importance,
                json.dumps(memory.source_commits),
                json.dumps(memory.file_paths), json.dumps(memory.tags),
                memory.accessed_at, memory.access_count,
                int(memory.active), memory.id,
            ),
        )

    def deactivate(self, memory_id: str) -> None:
        """Soft-delete a memory by marking it inactive."""
        self._db.execute(
            "UPDATE memories SET active = 0 WHERE id = ?",
            (memory_id,),
        )

    def touch(self, memory_id: str) -> None:
        """Record an access (updates accessed_at and increments access_count)."""
        now = self._now()
        self._db.execute(
            """UPDATE memories SET accessed_at = ?, access_count = access_count + 1
               WHERE id = ?""",
            (now, memory_id),
        )

    def query_by_file(
        self, path: str, *, limit: int = 20, min_importance: int = 0,
    ) -> list[Memory]:
        """Query memories associated with a file path or directory prefix."""
        pattern = f'%"{path}%'
        rows = self._db.query(
            """SELECT * FROM memories
               WHERE active = 1 AND file_paths LIKE ? AND importance >= ?
               ORDER BY importance DESC LIMIT ?""",
            (pattern, min_importance, limit),
        )
        return [self._row_to_memory(r) for r in rows]

    @staticmethod
    def _prepare_fts_query(query: str, match: str = "all") -> str:
        """Preprocess a natural-language query for FTS5 MATCH."""
        cleaned = re.sub(r'[^\w\s]', ' ', query)
        terms = [t for t in cleaned.split() if t]
        if not terms:
            return '""'
        joiner = " OR " if match == "any" else " AND "
        return joiner.join(f"{t}*" for t in terms)

    def search(
        self,
        query: str,
        *,
        memory_type: str | None = None,
        match: str = "all",
        min_importance: int = 0,
        limit: int = 20,
    ) -> list[Memory]:
        """Full-text search across memory summaries, types, and tags."""
        fts_query = self._prepare_fts_query(query, match)
        if memory_type:
            rows = self._db.query(
                """SELECT m.* FROM memories m
                   JOIN memories_fts f ON m.rowid = f.rowid
                   WHERE memories_fts MATCH ? AND m.active = 1
                     AND m.type = ? AND m.importance >= ?
                   ORDER BY rank LIMIT ?""",
                (fts_query, memory_type, min_importance, limit),
            )
        else:
            rows = self._db.query(
                """SELECT m.* FROM memories m
                   JOIN memories_fts f ON m.rowid = f.rowid
                   WHERE memories_fts MATCH ? AND m.active = 1
                     AND m.importance >= ?
                   ORDER BY rank LIMIT ?""",
                (fts_query, min_importance, limit),
            )
        return [self._row_to_memory(r) for r in rows]

    def list_all(self, *, active_only: bool = True, limit: int = 100) -> list[Memory]:
        """List all memories, optionally filtered by active status."""
        if active_only:
            rows = self._db.query(
                "SELECT * FROM memories WHERE active = 1 ORDER BY importance DESC LIMIT ?",
                (limit,),
            )
        else:
            rows = self._db.query(
                "SELECT * FROM memories ORDER BY importance DESC LIMIT ?",
                (limit,),
            )
        return [self._row_to_memory(r) for r in rows]

    def count(self, *, active_only: bool = True) -> int:
        """Count memories."""
        if active_only:
            row = self._db.query_one(
                "SELECT COUNT(*) as c FROM memories WHERE active = 1"
            )
        else:
            row = self._db.query_one("SELECT COUNT(*) as c FROM memories")
        return row["c"]

    def stats(self) -> dict:
        """Aggregate statistics about the memory store."""
        self._db.hold()
        try:
            total = self._db.query_one(
                "SELECT COUNT(*) as c FROM memories WHERE active = 1"
            )["c"]

            by_type = {}
            for row in self._db.query(
                "SELECT type, COUNT(*) as c FROM memories WHERE active = 1 GROUP BY type"
            ):
                by_type[row["type"]] = row["c"]

            confidence_stats = self._db.query_one(
                "SELECT AVG(confidence) as avg, MIN(confidence) as min, MAX(confidence) as max "
                "FROM memories WHERE active = 1"
            )

            avg_importance = self._db.query_one(
                "SELECT AVG(importance) as a FROM memories WHERE active = 1"
            )["a"]

            top_files: dict[str, int] = {}
            for row in self._db.query(
                "SELECT file_paths FROM memories WHERE active = 1"
            ):
                for f in json.loads(row["file_paths"]):
                    top_files[f] = top_files.get(f, 0) + 1
            top_files_sorted = sorted(top_files.items(), key=lambda x: x[1], reverse=True)[:10]
        finally:
            self._db.release()

        return {
            "total_memories": total,
            "by_type": by_type,
            "confidence": {
                "avg": round(confidence_stats["avg"], 1) if confidence_stats["avg"] else 0,
                "min": confidence_stats["min"] or 0,
                "max": confidence_stats["max"] or 0,
            },
            "avg_importance": round(avg_importance) if avg_importance else 0,
            "top_files": dict(top_files_sorted),
        }


class LinkStore:
    """CRUD for memory_links, bidirectional traversal (UUID-based)."""

    def __init__(self, db: Database):
        self._db = db

    @staticmethod
    def _now() -> str:
        return datetime.now(UTC).isoformat()

    def _row_to_link(self, row: sqlite3.Row) -> MemoryLink:
        return MemoryLink(
            id=row["id"],
            memory_id_a=row["memory_id_a"],
            memory_id_b=row["memory_id_b"],
            relationship=row["relationship"],
            strength=row["strength"],
            created_at=row["created_at"],
        )

    def create(self, link: MemoryLink) -> MemoryLink:
        """Insert a new link."""
        link.created_at = link.created_at or self._now()
        cursor = self._db.execute(
            """INSERT INTO memory_links
               (memory_id_a, memory_id_b, relationship, strength, created_at)
               VALUES (?, ?, ?, ?, ?)""",
            (link.memory_id_a, link.memory_id_b, link.relationship,
             link.strength, link.created_at),
        )
        link.id = cursor.lastrowid
        return link

    def get_links_for(self, memory_id: str) -> list[MemoryLink]:
        """Get all links involving a memory (bidirectional)."""
        rows = self._db.query(
            """SELECT * FROM memory_links
               WHERE memory_id_a = ? OR memory_id_b = ?""",
            (memory_id, memory_id),
        )
        return [self._row_to_link(r) for r in rows]

    def get_linked_ids(self, memory_id: str) -> list[str]:
        """Get UUIDs of all memories linked to the given memory."""
        links = self.get_links_for(memory_id)
        ids = []
        for link in links:
            other = link.memory_id_b if link.memory_id_a == memory_id else link.memory_id_a
            ids.append(other)
        return ids

    def delete_for_memory(self, memory_id: str) -> None:
        """Delete all links involving a memory."""
        self._db.execute(
            "DELETE FROM memory_links WHERE memory_id_a = ? OR memory_id_b = ?",
            (memory_id, memory_id),
        )

    def list_all(self, *, limit: int = 100) -> list[MemoryLink]:
        """List all links."""
        rows = self._db.query(
            "SELECT * FROM memory_links LIMIT ?", (limit,)
        )
        return [self._row_to_link(r) for r in rows]


class BuildMetaStore:
    """Operations on the build_meta table."""

    def __init__(self, db: Database):
        self._db = db

    def get_last(self) -> BuildMetaEntry | None:
        """Get the most recent build metadata entry."""
        row = self._db.query_one(
            "SELECT * FROM build_meta ORDER BY id DESC LIMIT 1"
        )
        if row is None:
            return None
        return BuildMetaEntry(
            id=row["id"],
            build_type=row["build_type"],
            commit_count=row["commit_count"],
            memory_count=row["memory_count"],
            built_at=row["built_at"],
        )

    def record(self, entry: BuildMetaEntry) -> BuildMetaEntry:
        """Record a new build run."""
        entry.built_at = entry.built_at or datetime.now(UTC).isoformat()
        cursor = self._db.execute(
            """INSERT INTO build_meta
               (build_type, commit_count, memory_count, built_at)
               VALUES (?, ?, ?, ?)""",
            (entry.build_type, entry.commit_count,
             entry.memory_count, entry.built_at),
        )
        entry.id = cursor.lastrowid
        return entry

    def list_all(self, *, limit: int = 20) -> list[BuildMetaEntry]:
        """List recent builds."""
        rows = self._db.query(
            "SELECT * FROM build_meta ORDER BY id DESC LIMIT ?", (limit,)
        )
        return [
            BuildMetaEntry(
                id=r["id"], build_type=r["build_type"],
                commit_count=r["commit_count"], memory_count=r["memory_count"],
                built_at=r["built_at"],
            )
            for r in rows
        ]
