"""Debug/inspect queries for AI visibility into raw data."""

import json
from dataclasses import asdict

from src.db import Database
from src.memory.stores import BuildMetaStore, LinkStore, MemoryStore


class Inspector:
    """Debug/inspect queries for AI visibility into raw data."""

    def __init__(
        self,
        db: Database,
        memory_store: MemoryStore,
        link_store: LinkStore,
        build_meta_store: BuildMetaStore,
    ):
        self._db = db
        self._memories = memory_store
        self._links = link_store
        self._build_meta = build_meta_store

    def inspect(self, query: str) -> str:
        """Route an inspect query to the appropriate handler."""
        parts = query.strip().lower().split()
        if not parts:
            return self._help()

        cmd = parts[0]
        args = parts[1:]

        handlers = {
            "tables": self._tables,
            "memories": self._all_memories,
            "memory": self._single_memory,
            "links": self._all_links,
            "builds": self._builds,
            "stats": self._stats,
            "schema": self._schema,
            "fts": self._fts_health,
            "help": self._help,
        }

        handler = handlers.get(cmd)
        if handler is None:
            return f"Unknown inspect command: {cmd}\n\n{self._help()}"

        return handler(*args)

    def _help(self, *_: str) -> str:
        return (
            "Inspect commands:\n"
            "  tables            — List all tables\n"
            "  memories          — Show all memories\n"
            "  memory <id>       — Show a specific memory with links\n"
            "  links             — Show all links\n"
            "  builds            — Show build history\n"
            "  stats             — Aggregate statistics\n"
            "  schema            — Show table schemas\n"
            "  fts               — FTS5 index health check\n"
            "  help              — This message"
        )

    def _tables(self, *_: str) -> str:
        rows = self._db.query(
            "SELECT name, type FROM sqlite_master WHERE type IN ('table', 'view') ORDER BY name"
        )
        return json.dumps([{"name": r["name"], "type": r["type"]} for r in rows], indent=2)

    def _all_memories(self, *_: str) -> str:
        memories = self._memories.list_all(active_only=False, limit=200)
        return json.dumps([m.to_dict() for m in memories], indent=2)

    def _single_memory(self, *args: str) -> str:
        if not args:
            return "Usage: memory <id>"
        memory_id = args[0]
        memory = self._memories.get(memory_id)
        if memory is None:
            return f"Memory {memory_id} not found"
        links = self._links.get_links_for(memory_id)
        return json.dumps({
            "memory": memory.to_dict(),
            "links": [asdict(lnk) for lnk in links],
        }, indent=2)

    def _all_links(self, *_: str) -> str:
        links = self._links.list_all(limit=200)
        return json.dumps([asdict(lnk) for lnk in links], indent=2)

    def _builds(self, *_: str) -> str:
        builds = self._build_meta.list_all()
        return json.dumps([asdict(b) for b in builds], indent=2)

    def _stats(self, *_: str) -> str:
        stats = self._memories.stats()
        last_build = self._build_meta.get_last()
        stats["last_build"] = asdict(last_build) if last_build else None
        return json.dumps(stats, indent=2)

    def _schema(self, *_: str) -> str:
        rows = self._db.query(
            "SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name"
        )
        return "\n\n".join(r["sql"] for r in rows)

    def _fts_health(self, *_: str) -> str:
        try:
            result = self._db.query_one(
                "SELECT COUNT(*) as c FROM memories_fts"
            )
            memory_count = self._memories.count(active_only=False)
            fts_count = result["c"]
            return json.dumps({
                "fts_rows": fts_count,
                "memory_rows": memory_count,
                "in_sync": fts_count == memory_count,
            }, indent=2)
        except Exception as e:
            return json.dumps({"error": str(e)}, indent=2)
