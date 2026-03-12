"""CRUD stores for memories, links, and build metadata."""

import json
import re
import sqlite3
from datetime import datetime, timezone
from typing import Optional

from src.models import Memory, MemoryLink, BuildMetaEntry
from src.db import Database


class MemoryStore:
    """CRUD for the memories table, file-based queries, FTS5 search."""

    def __init__(self, db: Database):
        self._db = db

    @staticmethod
    def _now() -> str:
        return datetime.now(timezone.utc).isoformat()

    def _row_to_memory(self, row: sqlite3.Row) -> Memory:
        return Memory(
            id=row["id"],
            summary=row["summary"],
            type=row["type"],
            confidence=row["confidence"],
            importance=row["importance"],
            source_commits=json.loads(row["source_commits"]),
            source_doc_refs=json.loads(row["source_doc_refs"]),
            files=json.loads(row["files"]),
            tags=json.loads(row["tags"]),
            created_at=row["created_at"],
            updated_at=row["updated_at"],
            accessed_at=row["accessed_at"],
            access_count=row["access_count"],
            active=bool(row["active"]),
        )

    def create(self, memory: Memory) -> Memory:
        """Insert a new memory and return it with its ID."""
        now = self._now()
        memory.created_at = memory.created_at or now
        memory.updated_at = memory.updated_at or now
        memory.accessed_at = memory.accessed_at or now

        cursor = self._db.conn.execute(
            """INSERT INTO memories
               (summary, type, confidence, importance, source_commits,
                source_doc_refs, files, tags, created_at, updated_at, accessed_at,
                access_count, active)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            (
                memory.summary, memory.type, memory.confidence, memory.importance,
                json.dumps(memory.source_commits), json.dumps(memory.source_doc_refs),
                json.dumps(memory.files), json.dumps(memory.tags),
                memory.created_at, memory.updated_at,
                memory.accessed_at, memory.access_count, int(memory.active),
            ),
        )
        self._db.conn.commit()
        memory.id = cursor.lastrowid
        return memory

    def get(self, memory_id: int) -> Optional[Memory]:
        """Get a single memory by ID."""
        row = self._db.conn.execute(
            "SELECT * FROM memories WHERE id = ?", (memory_id,)
        ).fetchone()
        if row is None:
            return None
        return self._row_to_memory(row)

    def update(self, memory: Memory) -> None:
        """Update an existing memory."""
        memory.updated_at = self._now()
        self._db.conn.execute(
            """UPDATE memories SET
               summary=?, type=?, confidence=?, importance=?, source_commits=?,
               source_doc_refs=?, files=?, tags=?, updated_at=?, accessed_at=?,
               access_count=?, active=?
               WHERE id=?""",
            (
                memory.summary, memory.type, memory.confidence, memory.importance,
                json.dumps(memory.source_commits), json.dumps(memory.source_doc_refs),
                json.dumps(memory.files), json.dumps(memory.tags),
                memory.updated_at, memory.accessed_at,
                memory.access_count, int(memory.active), memory.id,
            ),
        )
        self._db.conn.commit()

    def deactivate(self, memory_id: int) -> None:
        """Soft-delete a memory by marking it inactive."""
        self._db.conn.execute(
            "UPDATE memories SET active = 0, updated_at = ? WHERE id = ?",
            (self._now(), memory_id),
        )
        self._db.conn.commit()

    def touch(self, memory_id: int) -> None:
        """Record an access (updates accessed_at and increments access_count)."""
        now = self._now()
        self._db.conn.execute(
            """UPDATE memories SET accessed_at = ?, access_count = access_count + 1
               WHERE id = ?""",
            (now, memory_id),
        )
        self._db.conn.commit()

    def query_by_file(
        self, path: str, *, limit: int = 20, min_importance: float = 0.0,
    ) -> list[Memory]:
        """Query memories associated with a file path or directory prefix."""
        pattern = f'%"{path}%'
        rows = self._db.conn.execute(
            """SELECT * FROM memories
               WHERE active = 1 AND files LIKE ? AND importance >= ?
               ORDER BY importance DESC LIMIT ?""",
            (pattern, min_importance, limit),
        ).fetchall()
        return [self._row_to_memory(r) for r in rows]

    @staticmethod
    def _prepare_fts_query(query: str, match: str = "all") -> str:
        """Preprocess a natural-language query for FTS5 MATCH.

        Strips special characters, applies prefix matching, and joins
        terms with AND (match='all', default) or OR (match='any').
        """
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
        memory_type: Optional[str] = None,
        match: str = "all",
        min_importance: float = 0.0,
        limit: int = 20,
    ) -> list[Memory]:
        """Full-text search across memory summaries, types, and tags."""
        fts_query = self._prepare_fts_query(query, match)
        if memory_type:
            rows = self._db.conn.execute(
                """SELECT m.* FROM memories m
                   JOIN memories_fts f ON m.id = f.rowid
                   WHERE memories_fts MATCH ? AND m.active = 1
                     AND m.type = ? AND m.importance >= ?
                   ORDER BY rank LIMIT ?""",
                (fts_query, memory_type, min_importance, limit),
            ).fetchall()
        else:
            rows = self._db.conn.execute(
                """SELECT m.* FROM memories m
                   JOIN memories_fts f ON m.id = f.rowid
                   WHERE memories_fts MATCH ? AND m.active = 1
                     AND m.importance >= ?
                   ORDER BY rank LIMIT ?""",
                (fts_query, min_importance, limit),
            ).fetchall()
        return [self._row_to_memory(r) for r in rows]

    def list_all(self, *, active_only: bool = True, limit: int = 100) -> list[Memory]:
        """List all memories, optionally filtered by active status."""
        if active_only:
            rows = self._db.conn.execute(
                "SELECT * FROM memories WHERE active = 1 ORDER BY importance DESC LIMIT ?",
                (limit,),
            ).fetchall()
        else:
            rows = self._db.conn.execute(
                "SELECT * FROM memories ORDER BY importance DESC LIMIT ?",
                (limit,),
            ).fetchall()
        return [self._row_to_memory(r) for r in rows]

    def count(self, *, active_only: bool = True) -> int:
        """Count memories."""
        if active_only:
            row = self._db.conn.execute(
                "SELECT COUNT(*) as c FROM memories WHERE active = 1"
            ).fetchone()
        else:
            row = self._db.conn.execute("SELECT COUNT(*) as c FROM memories").fetchone()
        return row["c"]

    def stats(self) -> dict:
        """Aggregate statistics about the memory store."""
        c = self._db.conn
        total = c.execute("SELECT COUNT(*) as c FROM memories WHERE active = 1").fetchone()["c"]

        by_type = {}
        for row in c.execute(
            "SELECT type, COUNT(*) as c FROM memories WHERE active = 1 GROUP BY type"
        ).fetchall():
            by_type[row["type"]] = row["c"]

        by_confidence = {}
        for row in c.execute(
            "SELECT confidence, COUNT(*) as c FROM memories WHERE active = 1 GROUP BY confidence"
        ).fetchall():
            by_confidence[row["confidence"]] = row["c"]

        avg_importance = c.execute(
            "SELECT AVG(importance) as a FROM memories WHERE active = 1"
        ).fetchone()["a"]

        top_files: dict[str, int] = {}
        for row in c.execute(
            "SELECT files FROM memories WHERE active = 1"
        ).fetchall():
            for f in json.loads(row["files"]):
                top_files[f] = top_files.get(f, 0) + 1
        top_files_sorted = sorted(top_files.items(), key=lambda x: x[1], reverse=True)[:10]

        return {
            "total_memories": total,
            "by_type": by_type,
            "by_confidence": by_confidence,
            "avg_importance": round(avg_importance, 3) if avg_importance else 0.0,
            "top_files": dict(top_files_sorted),
        }

    def get_ids_accessed_since(self, since: str) -> set[int]:
        """Return IDs of memories accessed since the given ISO timestamp."""
        rows = self._db.conn.execute(
            "SELECT id FROM memories WHERE active = 1 AND accessed_at >= ?",
            (since,),
        ).fetchall()
        return {row["id"] for row in rows}

    def get_ids_for_commits(self, commit_hashes: list[str]) -> set[int]:
        """Return IDs of memories that reference any of the given commit hashes."""
        if not commit_hashes:
            return set()
        # Single query with OR conditions instead of one query per hash
        conditions = " OR ".join(["source_commits LIKE ?"] * len(commit_hashes))
        params = [f'%"{h}"%' for h in commit_hashes]
        rows = self._db.conn.execute(
            f"SELECT DISTINCT id FROM memories WHERE active = 1 AND ({conditions})",
            params,
        ).fetchall()
        return {row["id"] for row in rows}


class LinkStore:
    """CRUD for memory_links, bidirectional traversal."""

    def __init__(self, db: Database):
        self._db = db

    @staticmethod
    def _now() -> str:
        return datetime.now(timezone.utc).isoformat()

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
        cursor = self._db.conn.execute(
            """INSERT INTO memory_links
               (memory_id_a, memory_id_b, relationship, strength, created_at)
               VALUES (?, ?, ?, ?, ?)""",
            (link.memory_id_a, link.memory_id_b, link.relationship,
             link.strength, link.created_at),
        )
        self._db.conn.commit()
        link.id = cursor.lastrowid
        return link

    def get_links_for(self, memory_id: int) -> list[MemoryLink]:
        """Get all links involving a memory (bidirectional)."""
        rows = self._db.conn.execute(
            """SELECT * FROM memory_links
               WHERE memory_id_a = ? OR memory_id_b = ?""",
            (memory_id, memory_id),
        ).fetchall()
        return [self._row_to_link(r) for r in rows]

    def get_linked_ids(self, memory_id: int) -> list[int]:
        """Get IDs of all memories linked to the given memory."""
        links = self.get_links_for(memory_id)
        ids = []
        for link in links:
            other = link.memory_id_b if link.memory_id_a == memory_id else link.memory_id_a
            ids.append(other)
        return ids

    def delete_for_memory(self, memory_id: int) -> None:
        """Delete all links involving a memory."""
        self._db.conn.execute(
            "DELETE FROM memory_links WHERE memory_id_a = ? OR memory_id_b = ?",
            (memory_id, memory_id),
        )
        self._db.conn.commit()

    def list_all(self, *, limit: int = 100) -> list[MemoryLink]:
        """List all links."""
        rows = self._db.conn.execute(
            "SELECT * FROM memory_links LIMIT ?", (limit,)
        ).fetchall()
        return [self._row_to_link(r) for r in rows]


class BuildMetaStore:
    """Operations on the build_meta table."""

    def __init__(self, db: Database):
        self._db = db

    def get_last(self) -> Optional[BuildMetaEntry]:
        """Get the most recent build metadata entry."""
        row = self._db.conn.execute(
            "SELECT * FROM build_meta ORDER BY id DESC LIMIT 1"
        ).fetchone()
        if row is None:
            return None
        return BuildMetaEntry(
            id=row["id"],
            build_type=row["build_type"],
            last_commit=row["last_commit"],
            commit_count=row["commit_count"],
            memory_count=row["memory_count"],
            built_at=row["built_at"],
        )

    def record(self, entry: BuildMetaEntry) -> BuildMetaEntry:
        """Record a new build run."""
        entry.built_at = entry.built_at or datetime.now(timezone.utc).isoformat()
        cursor = self._db.conn.execute(
            """INSERT INTO build_meta
               (build_type, last_commit, commit_count, memory_count, built_at)
               VALUES (?, ?, ?, ?, ?)""",
            (entry.build_type, entry.last_commit, entry.commit_count,
             entry.memory_count, entry.built_at),
        )
        self._db.conn.commit()
        entry.id = cursor.lastrowid
        return entry

    def list_all(self, *, limit: int = 20) -> list[BuildMetaEntry]:
        """List recent builds."""
        rows = self._db.conn.execute(
            "SELECT * FROM build_meta ORDER BY id DESC LIMIT ?", (limit,)
        ).fetchall()
        return [
            BuildMetaEntry(
                id=r["id"], build_type=r["build_type"], last_commit=r["last_commit"],
                commit_count=r["commit_count"], memory_count=r["memory_count"],
                built_at=r["built_at"],
            )
            for r in rows
        ]
