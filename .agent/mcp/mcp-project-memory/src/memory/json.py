"""JSON file I/O for the canonical memory store.

Each memory is stored as an individual JSON file in data/memories/{uuid}.json.
Processed commit hashes are tracked in data/processed.json (sorted array).
"""

import hashlib
import json
from pathlib import Path

from src.memory.models import Memory


def _memories_dir(data_dir: Path) -> Path:
    d = data_dir / "memories"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _processed_path(data_dir: Path) -> Path:
    return data_dir / "processed.json"


# ── Memory file I/O ──


def write_memory(memory: Memory, data_dir: Path) -> Path:
    """Write a memory to data/memories/{uuid}.json. Returns the file path."""
    path = _memories_dir(data_dir) / f"{memory.id}.json"
    path.write_text(
        json.dumps(memory.to_json_dict(), indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    return path


def delete_memory(memory_id: str, data_dir: Path) -> bool:
    """Delete a memory's JSON file. Returns True if the file existed."""
    path = _memories_dir(data_dir) / f"{memory_id}.json"
    if path.exists():
        path.unlink()
        return True
    return False


def read_memory(memory_id: str, data_dir: Path) -> Memory | None:
    """Read a single memory by UUID. Returns None if not found."""
    path = _memories_dir(data_dir) / f"{memory_id}.json"
    if not path.exists():
        return None
    data = json.loads(path.read_text(encoding="utf-8"))
    return Memory.from_json_dict(data)


def read_all_memories(data_dir: Path) -> list[Memory]:
    """Read all active memory JSON files from data/memories/."""
    memories_path = _memories_dir(data_dir)
    memories: list[Memory] = []
    for path in sorted(memories_path.glob("*.json")):
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
            mem = Memory.from_json_dict(data)
            if mem.active:
                memories.append(mem)
        except (json.JSONDecodeError, KeyError) as e:
            import sys
            print(
                f"  warning: skipping corrupt memory file {path.name}: {e}",
                file=sys.stderr, flush=True,
            )
    return memories


def update_memory(memory: Memory, data_dir: Path) -> Path:
    """Overwrite an existing memory's JSON file with updated data."""
    return write_memory(memory, data_dir)


# ── Processed commits ──


def read_processed(data_dir: Path) -> set[str]:
    """Read the set of already-processed commit hashes."""
    path = _processed_path(data_dir)
    if not path.exists():
        return set()
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        return set(data) if isinstance(data, list) else set()
    except (json.JSONDecodeError, ValueError):
        return set()


def add_processed(hashes: set[str], data_dir: Path) -> None:
    """Add commit hashes to processed.json (merge + sort)."""
    existing = read_processed(data_dir)
    merged = sorted(existing | hashes)
    path = _processed_path(data_dir)
    path.write_text(
        json.dumps(merged, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )


# ── Fingerprinting (stale DB detection) ──


def compute_fingerprint(data_dir: Path) -> str:
    """Hash of sorted memory filenames + their mtime/size.

    Detects file additions, removals, AND content modifications.
    Uses mtime+size (fast) rather than content hashing (expensive).
    """
    memories_path = _memories_dir(data_dir)
    entries = []
    for p in sorted(memories_path.glob("*.json")):
        stat = p.stat()
        entries.append(f"{p.name}:{stat.st_mtime_ns}:{stat.st_size}")
    content = "\n".join(entries)
    return hashlib.sha256(content.encode()).hexdigest()[:16]
