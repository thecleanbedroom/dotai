"""Shared DB rebuild — reconstructs SQLite from JSON memory files.

Single source of truth for the rebuild-from-JSON logic, used by both
the MCP server (staleness check) and the BuildAgent (post-build).
"""

from pathlib import Path
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from src.config.settings import Settings

from src.db import Database
from src.memory import json as json_store
from src.memory.models import MemoryLink
from src.memory.stores import LinkStore, MemoryStore


def rebuild_db_from_json(
    db: Database,
    memory_store: MemoryStore,
    link_store: LinkStore,
    data_dir: Path,
    config: Optional["Settings"] = None,
) -> int:
    """Rebuild the SQLite DB from JSON memory files.

    Returns the number of memories loaded.
    Applies path filtering if config is provided.
    """
    db.hold()
    try:
        db.drop_all()
        db.init_schema()

        all_memories = json_store.read_all_memories(data_dir)

        # Apply path filtering if config is available
        if config:
            from src.path_filter import PathFilter
            path_filter = PathFilter.from_settings(config)
            if path_filter.patterns:
                before = len(all_memories)
                all_memories = [m for m in all_memories if path_filter.filter_memory(m)]
                dropped = before - len(all_memories)
                if dropped:
                    import sys
                    print(
                        f"  path filter: dropped {dropped} memories "
                        f"(all paths ignored)",
                        file=sys.stderr, flush=True,
                    )

        for mem in all_memories:
            memory_store.create(mem)

        # Restore links from memory JSON files
        valid_ids = {m.id for m in all_memories}
        for mem in all_memories:
            for link_data in mem.links:
                target = link_data.get("target", link_data.get("memory_id_b", ""))
                if target in valid_ids:
                    link = MemoryLink(
                        memory_id_a=mem.id,
                        memory_id_b=target,
                        relationship=link_data.get("relationship", "related_to"),
                        strength=float(link_data.get("strength", 0.5)),
                    )
                    link_store.create(link)

        fingerprint = json_store.compute_fingerprint(data_dir)
        db.set_fingerprint(fingerprint)
    finally:
        db.release()

    return len(all_memories)
