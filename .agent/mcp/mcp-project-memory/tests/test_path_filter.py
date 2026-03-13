"""Tests for PathFilter — glob-based path filtering."""

from src.memory.models import Memory, ParsedCommit
from src.path_filter import PathFilter


class TestIsIgnored:
    """Test PathFilter.is_ignored with various glob patterns."""

    def test_exact_match(self):
        pf = PathFilter([".agent/memory/data/*"])
        assert pf.is_ignored(".agent/memory/data/foo.json")

    def test_child_match(self):
        """Glob * matches children."""
        pf = PathFilter([".agent/memory/data/*"])
        assert pf.is_ignored(".agent/memory/data/foo.json")
        assert pf.is_ignored(".agent/memory/data/memories/bar.json")

    def test_no_match(self):
        pf = PathFilter([".agent/memory/data/*"])
        assert not pf.is_ignored(".agent/memory/src/build.py")
        assert not pf.is_ignored("src/main.py")

    def test_glob_star(self):
        pf = PathFilter(["*.json"])
        assert pf.is_ignored("foo.json")
        assert not pf.is_ignored("foo.py")

    def test_multiple_patterns(self):
        pf = PathFilter([".agent/memory/data/*", ".agent/skills/*"])
        assert pf.is_ignored(".agent/memory/data/foo")
        assert pf.is_ignored(".agent/skills/loki/SKILL.md")
        assert not pf.is_ignored(".agent/rules/core.md")

    def test_empty_patterns(self):
        pf = PathFilter([])
        assert not pf.is_ignored("anything.py")

    def test_empty_path(self):
        pf = PathFilter([".agent/memory/data/*"])
        assert not pf.is_ignored("")


class TestFilterCommit:
    """Test PathFilter.filter_commit stripping ignored files/diffs."""

    def _make_commit(self, files, diff=""):
        return ParsedCommit(
            hash="abc123",
            author="test",
            date="2026-01-01",
            message="test commit",
            body="",
            diff=diff,
            files=files,
        )

    def test_no_ignored_files(self):
        pf = PathFilter([".agent/memory/data/*"])
        commit = self._make_commit(["src/main.py", "src/lib.py"])
        result = pf.filter_commit(commit)
        assert result is not None
        assert result.files == ["src/main.py", "src/lib.py"]

    def test_all_files_ignored(self):
        pf = PathFilter([".agent/memory/data/*"])
        commit = self._make_commit([
            ".agent/memory/data/foo.json",
            ".agent/memory/data/bar.json",
        ])
        result = pf.filter_commit(commit)
        assert result is None

    def test_partial_ignore(self):
        pf = PathFilter([".agent/memory/data/*"])
        commit = self._make_commit([
            "src/main.py",
            ".agent/memory/data/foo.json",
        ])
        result = pf.filter_commit(commit)
        assert result is not None
        assert result.files == ["src/main.py"]

    def test_diff_sections_stripped(self):
        pf = PathFilter([".agent/memory/data/*"])
        diff = (
            "diff --git a/src/main.py b/src/main.py\n"
            "+good change\n"
            "diff --git a/.agent/memory/data/foo.json b/.agent/memory/data/foo.json\n"
            "+ignored change\n"
            "diff --git a/src/lib.py b/src/lib.py\n"
            "+another good change"
        )
        commit = self._make_commit(
            ["src/main.py", ".agent/memory/data/foo.json", "src/lib.py"],
            diff=diff,
        )
        result = pf.filter_commit(commit)
        assert result is not None
        assert ".agent/memory/data/foo.json" not in result.files
        assert "+ignored change" not in result.diff
        assert "+good change" in result.diff
        assert "+another good change" in result.diff

    def test_empty_files_commit(self):
        """Commit with no files is kept as-is."""
        pf = PathFilter([".agent/memory/data/*"])
        commit = self._make_commit([])
        result = pf.filter_commit(commit)
        assert result is not None


class TestFilterMemory:
    """Test PathFilter.filter_memory for orphan drop-out."""

    def test_no_paths_kept(self):
        """Memories with no file_paths are always kept."""
        pf = PathFilter([".agent/memory/data/*"])
        mem = Memory(summary="test", file_paths=[])
        assert pf.filter_memory(mem)

    def test_all_paths_ignored(self):
        pf = PathFilter([".agent/memory/data/*"])
        mem = Memory(summary="test", file_paths=[
            ".agent/memory/data/foo.json",
            ".agent/memory/data/bar.json",
        ])
        assert not pf.filter_memory(mem)

    def test_mixed_paths_kept(self):
        pf = PathFilter([".agent/memory/data/*"])
        mem = Memory(summary="test", file_paths=[
            "src/main.py",
            ".agent/memory/data/foo.json",
        ])
        assert pf.filter_memory(mem)

    def test_no_ignored_paths(self):
        pf = PathFilter([".agent/memory/data/*"])
        mem = Memory(summary="test", file_paths=["src/main.py"])
        assert pf.filter_memory(mem)


class TestFromSettings:
    """Test PathFilter.from_settings parses comma-separated patterns."""

    def test_default(self):
        from src.config.settings import Settings
        settings = Settings()
        pf = PathFilter.from_settings(settings)
        assert ".agent/memory/data/*" in pf.patterns

    def test_custom(self):
        from src.config.settings import Settings
        settings = Settings(memory_ignore_paths=".agent/skills/*,*.lock")
        pf = PathFilter.from_settings(settings)
        assert pf.is_ignored(".agent/skills/foo/SKILL.md")
        assert pf.is_ignored("package-lock.lock")

    def test_empty(self):
        from src.config.settings import Settings
        settings = Settings(memory_ignore_paths="")
        pf = PathFilter.from_settings(settings)
        assert pf.patterns == []
