"""Centralized path filtering — decides which files are ignored during builds."""

from fnmatch import fnmatch
from typing import TYPE_CHECKING

from src.utils import split_diff_by_file

if TYPE_CHECKING:
    from src.config.settings import Settings

from src.memory.models import Memory, ParsedCommit


class PathFilter:
    """Glob-based path filter. Used by build to skip irrelevant files/memories.

    Patterns are matched via fnmatch against the full relative path.
    Standard glob syntax: * matches anything, ? matches one char.
    Examples:
        .agent/memory/data/*  — matches all children
        *.lock                — matches any .lock file
        .agent/skills/*       — matches all skill files
    """

    def __init__(self, patterns: list[str]) -> None:
        self._patterns: list[str] = [p.strip() for p in patterns if p.strip()]

    @classmethod
    def from_settings(cls, settings: "Settings") -> "PathFilter":
        """Parse comma-separated filter_ignore_paths from Settings."""
        raw = settings.filter_ignore_paths()
        patterns = [p.strip() for p in raw.split(",") if p.strip()]
        return cls(patterns)

    @property
    def patterns(self) -> list[str]:
        """Return the configured patterns."""
        return list(self._patterns)

    def is_ignored(self, path: str) -> bool:
        """Return True if path matches any ignore pattern."""
        path = path.strip()
        if not path:
            return False
        return any(fnmatch(path, pattern) for pattern in self._patterns)

    def filter_commit(self, commit: ParsedCommit) -> ParsedCommit | None:
        """Return a pruned commit with ignored files/diffs stripped.

        Returns None if all files are ignored (commit should be skipped).
        """
        kept_files = [f for f in commit.files if not self.is_ignored(f)]
        if not kept_files and commit.files:
            return None

        # Strip ignored diff sections
        if commit.diff:
            by_file = split_diff_by_file(commit.diff)
            kept_diffs = [
                diff_text for path, diff_text in by_file.items()
                if not self.is_ignored(path)
            ]
            filtered_diff = "\n".join(kept_diffs)
        else:
            filtered_diff = commit.diff

        return ParsedCommit(
            hash=commit.hash,
            author=commit.author,
            date=commit.date,
            message=commit.message,
            body=commit.body,
            diff=filtered_diff,
            files=kept_files,
            trailers=commit.trailers,
        )

    def filter_memory(self, memory: Memory) -> bool:
        """Return True if the memory should be kept.

        A memory is dropped only if it has file_paths AND every one is ignored.
        Memories with no file_paths are always kept.
        Memories with at least one non-ignored path are kept.
        """
        if not memory.file_paths:
            return True
        return any(not self.is_ignored(p) for p in memory.file_paths)
