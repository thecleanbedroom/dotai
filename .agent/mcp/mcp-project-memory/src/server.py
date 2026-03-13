"""MCP server with tool registration and stdio transport."""

import json
import sys
from collections.abc import Callable
from dataclasses import asdict
from pathlib import Path
from urllib.parse import urlparse

from src.memory import json as json_store

_CAVEAT = (
    "\u26a0\ufe0f Memories are historical context extracted from git history"
    " \u2014 always verify against current code before acting on them."
)

_SERVER_INSTRUCTIONS = """Project Memory \u2014 persistent knowledge extracted from git history.

Before working on any area of the codebase, call `search_file_memory_by_path`
with the file path or `search_project_memory_by_topic` with relevant terms.
This surfaces past decisions, known bugs, debt, and conventions that are NOT
visible in the code itself. Prevents repeating past mistakes and contradicting
established decisions.

Search tips:
- Use short, focused queries (2-3 terms). Avoid long compound phrases.
- Run multiple small searches rather than one big query.
- Default match is "any" (OR) for broad discovery. Use match="all" to narrow.
- If a broad query returns too many results, add a type filter or raise min_importance.

Results are historical context \u2014 always verify against current code."""


class McpServer:
    """MCP SDK server with tool registration and stdio transport.

    Uses MCP roots to detect the project directory at runtime.
    Components (stores, build agent, etc.) are created lazily on
    the first tool call via a factory function.

    On first access, checks if the SQLite DB is stale relative to
    the JSON memory files and rebuilds if needed.
    """

    def __init__(self, component_factory: Callable):
        self._factory = component_factory
        self._components: dict | None = None
        self._db_checked = False

    async def _ensure_components(self, ctx) -> dict:
        """Lazily resolve project root from MCP roots and create components."""
        if self._components is not None:
            if not self._db_checked:
                self._check_db_freshness()
            return self._components

        project_root = None
        try:
            result = await ctx.session.list_roots()
            if result.roots:
                uri = str(result.roots[0].uri)
                parsed = urlparse(uri)
                if parsed.scheme == "file":
                    project_root = parsed.path
        except Exception as e:
            print(
                f"  warning: could not resolve MCP roots: {e}",
                file=sys.stderr, flush=True,
            )

        # Override the single source of truth so all modules (db, llm, git)
        # use the MCP client's project, not where the server code lives.
        if project_root:
            from src.utils import set_root_dir
            set_root_dir(project_root)

        project_name = project_root.rstrip("/").rsplit("/", 1)[-1] if project_root else "unknown"
        print(
            f"  project: {project_name} ({project_root})",
            file=sys.stderr, flush=True,
        )

        self._components = self._factory(project_root=project_root)
        if self._components is None:
            raise RuntimeError("Component factory returned None")

        self._check_db_freshness()
        return self._components

    def _check_db_freshness(self) -> None:
        """Check if the DB is stale vs JSON files and rebuild if needed."""
        self._db_checked = True
        try:
            from src.utils import data_dir
            dd = data_dir()
            if not dd.exists():
                return

            assert self._components is not None
            db = self._components["db"]
            current_fp = json_store.compute_fingerprint(dd)
            stored_fp = db.get_fingerprint()

            if stored_fp != current_fp:
                print(
                    "  db stale (fingerprint mismatch), rebuilding from JSON...",
                    file=sys.stderr, flush=True,
                )
                self._rebuild_db_from_json(dd)
        except Exception as e:
            print(
                f"  warning: db freshness check failed: {e}",
                file=sys.stderr, flush=True,
            )

    def _rebuild_db_from_json(self, data_dir: Path) -> None:
        """Rebuild the SQLite DB from JSON memory files."""
        from src.config.settings import Settings
        from src.memory.db_rebuild import rebuild_db_from_json

        assert self._components is not None
        count = rebuild_db_from_json(
            self._components["db"],
            self._components["memory_store"],
            self._components["link_store"],
            data_dir,
            config=Settings.load(),
        )

        print(
            f"  rebuilt db: {count} memories loaded",
            file=sys.stderr, flush=True,
        )

    def run(self) -> None:
        """Start the MCP server over stdio."""
        from mcp.server.fastmcp import Context, FastMCP

        mcp = FastMCP("project-memory", instructions=_SERVER_INSTRUCTIONS)
        server = self  # capture for closures

        from mcp.types import ToolAnnotations

        _RO = ToolAnnotations(readOnlyHint=True)

        @mcp.tool(annotations=_RO)
        async def search_file_memory_by_path(
            ctx: Context,
            path: str,
            min_importance: int = 0,
            limit: int = 20,
        ) -> str:
            """Call before modifying a file. Returns past decisions, known bugs,
            debt, and patterns associated with the file path. Supports exact
            file paths and directory prefixes.

            Results are historical context — always verify against current code.
            """
            c = await server._ensure_components(ctx)
            memories = c["memory_store"].query_by_file(
                path,
                limit=limit,
                min_importance=min_importance,
            )
            for m in memories:
                c["memory_store"].touch(m.id)
            result = [m.to_dict() for m in memories]
            return json.dumps({"caveat": _CAVEAT, "memories": result}, indent=2)

        @mcp.tool(annotations=_RO)
        async def search_project_memory_by_topic(
            ctx: Context,
            query: str,
            type: str = "",
            match: str = "any",
            min_importance: int = 0,
            limit: int = 20,
        ) -> str:
            """Search project memory by topic. Returns decisions, conventions,
            and patterns not visible in the code. Call when researching an area
            or starting work on a feature.

            Use short queries (2-3 terms). Run multiple searches rather than
            one compound query.

            match parameter:
            - "any" (default): returns memories matching ANY keyword (OR).
              Best for exploratory searches and broad discovery.
            - "all": returns only memories matching EVERY keyword (AND).
              Use to narrow when "any" returns too many results.

            Results are historical context — always verify against current code.
            """
            c = await server._ensure_components(ctx)
            memories = c["memory_store"].search(
                query,
                memory_type=type or None,
                match=match,
                min_importance=min_importance,
                limit=limit,
            )
            for m in memories:
                c["memory_store"].touch(m.id)
            result = [m.to_dict() for m in memories]
            return json.dumps({"caveat": _CAVEAT, "memories": result}, indent=2)

        @mcp.tool(annotations=_RO)
        async def recall_memory(
            ctx: Context, memory_id: str, include_links: bool = True,
        ) -> str:
            """Retrieve a specific memory by ID with full detail and linked
            memories. Use to drill into search results and see connections.

            Results are historical context — always verify against current code.
            """
            c = await server._ensure_components(ctx)
            memory = c["memory_store"].get(memory_id)
            if memory is None:
                return json.dumps({"error": f"Memory {memory_id} not found"})

            c["memory_store"].touch(memory.id)
            result = memory.to_dict()

            if include_links:
                links = c["link_store"].get_links_for(memory.id)
                result["links"] = [asdict(lnk) for lnk in links]
                linked_ids = c["link_store"].get_linked_ids(memory.id)
                linked = c["memory_store"].get_many(linked_ids)
                result["linked_memories"] = [m.to_dict() for m in linked]

            return json.dumps({"caveat": _CAVEAT, "memory": result}, indent=2)

        @mcp.tool(annotations=_RO)
        async def project_memory_overview(ctx: Context) -> str:
            """Overview of project memory — total memory count, breakdown by
            type (decision, pattern, convention, context, debt), breakdown by
            confidence (high, medium, low), average importance score, top 10
            most-referenced files, and last build info (date, commits processed,
            memory count).
            """
            c = await server._ensure_components(ctx)
            stats = c["memory_store"].stats()
            last_build = c["build_meta_store"].get_last()
            stats["last_build"] = asdict(last_build) if last_build else None
            return json.dumps(stats, indent=2)

        @mcp.tool(annotations=_RO)
        async def memory_inspect(ctx: Context, query: str) -> str:
            """Debug/inspect the memory system internals.

            Commands: tables, memories, memory <id>, links, builds, stats, schema, fts, help
            """
            c = await server._ensure_components(ctx)
            return c["inspector"].inspect(query)

        @mcp.prompt()
        async def briefing(ctx: Context) -> list[dict]:
            """Load key project context — top decisions, patterns, and conventions from project memory."""
            c = await server._ensure_components(ctx)
            memories = c["memory_store"].list_all(limit=20)
            if not memories:
                return [{"role": "user", "content": "No project memories found. Run a build first."}]
            lines = [f"- **[{m.type}]** (importance: {m.importance}) {m.summary}" for m in memories]
            summary = "\n".join(lines)
            return [{"role": "user", "content": (
                f"## Project Memory Briefing\n\n"
                f"{_CAVEAT}\n\n"
                f"{summary}"
            )}]

        mcp.run(transport="stdio")

    def cleanup(self) -> None:
        """Close the database connection if components were created."""
        if self._components and "db" in self._components:
            self._components["db"].close()
