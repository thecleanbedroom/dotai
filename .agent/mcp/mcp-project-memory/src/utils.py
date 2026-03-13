"""Shared utility functions."""

import subprocess
from pathlib import Path

from src.config.internal import InternalSettings

# Module-level cache. Set by _detect_root_dir() on first call.
# The MCP server overrides this when a client connects to a different project.
_ROOT_DIR: Path | None = None


def _detect_root_dir() -> Path:
    """Detect project root via git rev-parse.

    Falls back to deriving from this file's location if not in a git repo.
    """
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True, text=True, timeout=5,
        )
        if result.returncode == 0:
            return Path(result.stdout.strip())
    except Exception:
        pass
    # Fallback: src/utils.py → src/ → memory/ → .agent/ → project root
    return Path(__file__).resolve().parent.parent.parent.parent


def root_dir() -> Path:
    """Git repo root for the current project.

    Cached after first call. MCP server overrides via set_root_dir().
    """
    global _ROOT_DIR
    if _ROOT_DIR is None:
        _ROOT_DIR = _detect_root_dir()
    return _ROOT_DIR


def set_root_dir(path: str | Path) -> None:
    """Override the project root (used by MCP server for multi-project support)."""
    global _ROOT_DIR
    _ROOT_DIR = Path(path)


def project_dir() -> Path:
    """Root directory of the memory system within the project.

    Derived from this file's location: src/utils.py → src/ → memory/.
    Uses non-resolved path to preserve symlinks.
    """
    return Path(__file__).parent.parent


def data_dir() -> Path:
    """Runtime data directory (memories, DB, build logs).

    Single source of truth — every module that needs the data path
    should call this instead of constructing the path independently.
    """
    d = project_dir() / "data"
    d.mkdir(parents=True, exist_ok=True)
    return d


def estimate_tokens(text: str) -> int:
    """Estimate token count from character length."""
    return max(len(text) // InternalSettings.token_chars_per_token(), 1)


def split_diff_by_file(diff: str) -> dict[str, str]:
    """Split a unified diff into per-file chunks.

    Returns a dict mapping file path → diff text for that file.
    """
    result: dict[str, str] = {}
    if not diff:
        return result
    current_file = ""
    current_lines: list[str] = []
    for line in diff.split("\n"):
        if line.startswith("diff --git "):
            if current_file and current_lines:
                result[current_file] = "\n".join(current_lines)
            parts = line.split(" b/", 1)
            current_file = parts[1] if len(parts) > 1 else ""
            current_lines = [line]
        else:
            current_lines.append(line)
    if current_file and current_lines:
        result[current_file] = "\n".join(current_lines)
    return result


def filter_binary_diffs(diff: str) -> str:
    """Keep text diffs, strip binary diffs."""
    by_file = split_diff_by_file(diff)
    kept: list[str] = []
    for file_diff in by_file.values():
        if "Binary files" not in file_diff or "differ" not in file_diff:
            kept.append(file_diff)
    return "\n".join(kept)
