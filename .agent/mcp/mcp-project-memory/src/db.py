"""SQLite connection management and schema creation.

The database is a disposable runtime cache, rebuilt from JSON memory files
on demand.  It is gitignored — never committed to version control.

Connection lifecycle is fully managed here:
  - execute() / executescript(): auto-commit + auto-close
  - query() / query_one(): auto-close (no commit needed)
  - hold() / release(): keep connection open for bulk operations
"""

import sqlite3
from pathlib import Path


class Database:
    """SQLite connection — owns full lifecycle: open, commit, close.

    By default, every operation opens a connection, does its work,
    commits (for writes), and closes.  For bulk operations (rebuild),
    call hold() first and release() when done.
    """

    SCHEMA_VERSION = 2

    def __init__(self, db_path: str | None = None):
        if db_path is None:
            from src.utils import data_dir
            db_path = str(data_dir() / "project_memory.db")
        self.db_path = db_path
        self._conn: sqlite3.Connection | None = None
        self._held = False
        self._schema_ready = False

    # ── Private connection management ──

    def _open(self) -> sqlite3.Connection:
        """Open connection if needed.  Ensures schema on first open."""
        if self._conn is None:
            self._conn = sqlite3.connect(self.db_path)
            self._conn.row_factory = sqlite3.Row
            self._conn.execute("PRAGMA journal_mode=WAL")
            self._conn.execute("PRAGMA synchronous=NORMAL")
            self._conn.execute("PRAGMA cache_size=-64000")
            self._conn.execute("PRAGMA temp_store=MEMORY")
            self._conn.execute("PRAGMA mmap_size=268435456")
            self._conn.execute("PRAGMA foreign_keys=ON")
            if not self._schema_ready:
                self._init_schema()
                self._schema_ready = True
        return self._conn

    def _auto_close(self, *, commit: bool = False) -> None:
        """Commit (if requested) and close — unless held."""
        if self._held:
            if commit and self._conn:
                self._conn.commit()
            return
        if self._conn is not None:
            if commit:
                self._conn.commit()
            self._conn.close()
            self._conn = None

    # ── Public data interface ──

    def execute(self, sql: str, params: tuple = ()) -> sqlite3.Cursor:
        """Execute a write statement.  Auto-commits + auto-closes."""
        conn = self._open()
        cursor = conn.execute(sql, params)
        self._auto_close(commit=True)
        return cursor

    def query(self, sql: str, params: tuple = ()) -> list[sqlite3.Row]:
        """Execute a read query.  Returns all rows.  Auto-closes."""
        conn = self._open()
        rows = conn.execute(sql, params).fetchall()
        self._auto_close()
        return rows

    def query_one(self, sql: str, params: tuple = ()) -> sqlite3.Row | None:
        """Execute a read query.  Returns first row or None.  Auto-closes."""
        conn = self._open()
        row = conn.execute(sql, params).fetchone()
        self._auto_close()
        return row

    def executescript(self, sql: str) -> None:
        """Execute a multi-statement script.  Auto-commits + auto-closes."""
        conn = self._open()
        conn.executescript(sql)
        self._auto_close(commit=True)

    # ── Bulk operation scope ──

    def hold(self) -> None:
        """Keep connection open across multiple operations.

        Call release() when done to commit + close.
        """
        self._open()
        self._held = True

    def release(self) -> None:
        """Commit + close after a held session."""
        self._held = False
        self._auto_close(commit=True)

    # ── Context manager (convenience alias for hold/release) ──

    def __enter__(self):
        self.hold()
        return self

    def __exit__(self, *exc):
        self.release()

    # ── Schema ──

    def _init_schema(self) -> None:
        """Create all tables, indexes, FTS5, and triggers if they don't exist."""
        c = self._conn
        c.executescript("""
            CREATE TABLE IF NOT EXISTS memories (
                id              TEXT PRIMARY KEY,
                summary         TEXT NOT NULL,
                type            TEXT NOT NULL,
                confidence      INTEGER NOT NULL DEFAULT 0,
                importance      INTEGER NOT NULL DEFAULT 50,
                source_commits  TEXT NOT NULL DEFAULT '[]',
                file_paths      TEXT NOT NULL DEFAULT '[]',
                tags            TEXT NOT NULL DEFAULT '[]',
                created_at      TEXT NOT NULL,
                accessed_at     TEXT NOT NULL DEFAULT '',
                access_count    INTEGER NOT NULL DEFAULT 0,
                active          INTEGER NOT NULL DEFAULT 1
            );

            CREATE TABLE IF NOT EXISTS memory_links (
                id            INTEGER PRIMARY KEY AUTOINCREMENT,
                memory_id_a   TEXT NOT NULL REFERENCES memories(id),
                memory_id_b   TEXT NOT NULL REFERENCES memories(id),
                relationship  TEXT NOT NULL,
                strength      REAL NOT NULL DEFAULT 0.5,
                created_at    TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS build_meta (
                id              INTEGER PRIMARY KEY AUTOINCREMENT,
                build_type      TEXT NOT NULL,
                commit_count    INTEGER NOT NULL,
                memory_count    INTEGER NOT NULL,
                built_at        TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS db_meta (
                key   TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
            CREATE INDEX IF NOT EXISTS idx_memories_active ON memories(active);
            CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
            CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(confidence);
            CREATE INDEX IF NOT EXISTS idx_memory_links_a ON memory_links(memory_id_a);
            CREATE INDEX IF NOT EXISTS idx_memory_links_b ON memory_links(memory_id_b);
        """)

        # FTS5
        try:
            c.execute("""
                CREATE VIRTUAL TABLE memories_fts USING fts5(
                    summary, type, tags,
                    content=memories,
                    content_rowid=rowid
                )
            """)
        except sqlite3.OperationalError:
            pass  # Already exists

        # FTS sync triggers
        for trigger_sql in [
            """
            CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
                INSERT INTO memories_fts(rowid, summary, type, tags)
                VALUES (new.rowid, new.summary, new.type, new.tags);
            END
            """,
            """
            CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
                INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
                VALUES ('delete', old.rowid, old.summary, old.type, old.tags);
            END
            """,
            """
            CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
                INSERT INTO memories_fts(memories_fts, rowid, summary, type, tags)
                VALUES ('delete', old.rowid, old.summary, old.type, old.tags);
                INSERT INTO memories_fts(rowid, summary, type, tags)
                VALUES (new.rowid, new.summary, new.type, new.tags);
            END
            """,
        ]:
            c.execute(trigger_sql)

        c.commit()

    def drop_all(self) -> None:
        """Drop all tables for a full rebuild.  Requires hold()."""
        conn = self._open()
        conn.executescript("""
            DROP TABLE IF EXISTS memory_links;
            DROP TABLE IF EXISTS memories_fts;
            DROP TABLE IF EXISTS memories;
            DROP TABLE IF EXISTS build_meta;
            DROP TABLE IF EXISTS db_meta;
        """)
        conn.commit()
        self._schema_ready = False

    def init_schema(self) -> None:
        """Public: re-initialize schema (after drop_all)."""
        self._init_schema()
        self._schema_ready = True

    # ── Fingerprint / staleness ──

    def get_fingerprint(self) -> str | None:
        """Read the stored JSON fingerprint (or None if not set)."""
        try:
            row = self.query_one(
                "SELECT value FROM db_meta WHERE key = 'json_fingerprint'"
            )
            return row["value"] if row else None
        except sqlite3.OperationalError:
            return None

    def set_fingerprint(self, fingerprint: str) -> None:
        """Store the current JSON fingerprint."""
        self.execute(
            "INSERT OR REPLACE INTO db_meta (key, value) VALUES ('json_fingerprint', ?)",
            (fingerprint,),
        )
