"""SQLite connection management and schema creation."""

import sqlite3
from pathlib import Path
from typing import Optional


class Database:
    """SQLite connection management and schema creation."""

    SCHEMA_VERSION = 1

    def __init__(self, db_path: Optional[str] = None):
        if db_path is None:
            data_dir = Path.cwd() / ".agent" / "memory" / "data"
            data_dir.mkdir(parents=True, exist_ok=True)
            db_path = str(data_dir / "project_memory.db")
        self.db_path = db_path
        self._conn: Optional[sqlite3.Connection] = None

    @property
    def conn(self) -> sqlite3.Connection:
        if self._conn is None:
            self._conn = sqlite3.connect(self.db_path)
            self._conn.row_factory = sqlite3.Row
            # Performance tuning
            self._conn.execute("PRAGMA journal_mode=WAL")
            self._conn.execute("PRAGMA synchronous=NORMAL")  # Safe with WAL
            self._conn.execute("PRAGMA cache_size=-64000")   # 64MB cache
            self._conn.execute("PRAGMA temp_store=MEMORY")
            self._conn.execute("PRAGMA mmap_size=268435456") # 256MB mmap
            self._conn.execute("PRAGMA foreign_keys=ON")
        return self._conn

    def close(self) -> None:
        if self._conn is not None:
            # Checkpoint WAL to clean up -shm and -wal files on exit.
            # TRUNCATE mode flushes WAL to the main DB and resets both files.
            try:
                self._conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
            except Exception:
                pass  # Best-effort cleanup
            self._conn.close()
            self._conn = None

    def init_schema(self) -> None:
        """Create all tables, indexes, FTS5, and triggers if they don't exist."""
        c = self.conn
        c.executescript("""
            CREATE TABLE IF NOT EXISTS memories (
                id              INTEGER PRIMARY KEY AUTOINCREMENT,
                summary         TEXT NOT NULL,
                type            TEXT NOT NULL,
                confidence      TEXT NOT NULL,
                importance      REAL NOT NULL DEFAULT 0.5,
                source_commits  TEXT NOT NULL DEFAULT '[]',
                source_doc_refs TEXT NOT NULL DEFAULT '[]',
                files           TEXT NOT NULL DEFAULT '[]',
                tags            TEXT NOT NULL DEFAULT '[]',
                created_at      TEXT NOT NULL,
                updated_at      TEXT NOT NULL,
                accessed_at     TEXT NOT NULL,
                access_count    INTEGER NOT NULL DEFAULT 0,
                active          INTEGER NOT NULL DEFAULT 1
            );

            CREATE TABLE IF NOT EXISTS memory_links (
                id            INTEGER PRIMARY KEY AUTOINCREMENT,
                memory_id_a   INTEGER NOT NULL REFERENCES memories(id),
                memory_id_b   INTEGER NOT NULL REFERENCES memories(id),
                relationship  TEXT NOT NULL,
                strength      REAL NOT NULL DEFAULT 0.5,
                created_at    TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS build_meta (
                id              INTEGER PRIMARY KEY AUTOINCREMENT,
                build_type      TEXT NOT NULL,
                last_commit     TEXT NOT NULL,
                commit_count    INTEGER NOT NULL,
                memory_count    INTEGER NOT NULL,
                built_at        TEXT NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
            CREATE INDEX IF NOT EXISTS idx_memories_active ON memories(active);
            CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
            CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(confidence);
            CREATE INDEX IF NOT EXISTS idx_memory_links_a ON memory_links(memory_id_a);
            CREATE INDEX IF NOT EXISTS idx_memory_links_b ON memory_links(memory_id_b);
        """)

        # FTS5 — created separately because IF NOT EXISTS isn't supported in executescript
        # for virtual tables in all SQLite versions
        try:
            c.execute("""
                CREATE VIRTUAL TABLE memories_fts USING fts5(
                    summary, type, tags,
                    content=memories,
                    content_rowid=id
                )
            """)
        except sqlite3.OperationalError:
            pass  # Already exists

        # FTS sync triggers
        for trigger_sql in [
            """
            CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
                INSERT INTO memories_fts(rowid, summary, type, tags)
                VALUES (new.id, new.summary, new.type, new.tags);
            END
            """,
            """
            CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
                INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
                VALUES ('delete', old.id, old.summary, old.type, old.tags);
            END
            """,
            """
            CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
                INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
                VALUES ('delete', old.id, old.summary, old.type, old.tags);
                INSERT INTO memories_fts(rowid, summary, type, tags)
                VALUES (new.id, new.summary, new.type, new.tags);
            END
            """,
        ]:
            c.execute(trigger_sql)

        c.commit()

    def drop_all(self) -> None:
        """Drop all tables for a full rebuild."""
        c = self.conn
        c.executescript("""
            DROP TABLE IF EXISTS memory_links;
            DROP TABLE IF EXISTS memories_fts;
            DROP TABLE IF EXISTS memories;
            DROP TABLE IF EXISTS build_meta;
        """)
        c.commit()
