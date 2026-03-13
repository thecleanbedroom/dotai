"""Tests for src.build.batching — split_diff_by_file and related."""

import pytest

from src.build.batching import BatchPlanner
from src.utils import split_diff_by_file
from src.memory.models import ParsedCommit


class TestSplitDiffByFile:
    """Test the shared split_diff_by_file utility."""

    def test_empty_diff(self):
        assert split_diff_by_file("") == {}

    def test_single_file_diff(self):
        diff = (
            "diff --git a/foo.py b/foo.py\n"
            "--- a/foo.py\n"
            "+++ b/foo.py\n"
            "@@ -1 +1 @@\n"
            "-old\n"
            "+new"
        )
        result = split_diff_by_file(diff)
        assert "foo.py" in result
        assert "+new" in result["foo.py"]

    def test_multi_file_diff(self):
        diff = (
            "diff --git a/a.py b/a.py\n"
            "+line_a\n"
            "diff --git a/b.py b/b.py\n"
            "+line_b"
        )
        result = split_diff_by_file(diff)
        assert len(result) == 2
        assert "a.py" in result
        assert "b.py" in result
        assert "+line_a" in result["a.py"]
        assert "+line_b" in result["b.py"]

    def test_no_diff_headers(self):
        diff = "just some text without diff headers"
        result = split_diff_by_file(diff)
        assert result == {}


class TestEstimateCommitTokens:
    """Test token estimation."""

    def test_minimal_commit(self):
        commit = ParsedCommit(
            hash="abc123", author="test", date="2025-01-01",
            message="fix", body="", diff="", files=[], trailers={},
        )
        tokens = BatchPlanner.estimate_commit_tokens(commit)
        assert tokens >= 1

    def test_large_commit(self):
        commit = ParsedCommit(
            hash="abc123", author="test", date="2025-01-01",
            message="big change", body="x" * 10000,
            diff="y" * 20000, files=["a.py", "b.py"],
            trailers={"key": "val"},
        )
        tokens = BatchPlanner.estimate_commit_tokens(commit)
        assert tokens > 7000  # ~30k chars / 4


class TestMakeBatches:
    """Test batch planning."""

    @pytest.fixture
    def planner(self):
        from src.config.settings import Settings
        return BatchPlanner(Settings.load())

    def _commit(self, size=100):
        return ParsedCommit(
            hash="a" * 8, author="x", date="d",
            message="m", body="b" * size, diff="d" * size,
            files=["f.py"], trailers={},
        )

    def test_single_batch(self, planner):
        commits = [self._commit(10) for _ in range(3)]
        batches = planner.make_batches(commits, budget=100000)
        assert len(batches) == 1
        assert len(batches[0]) == 3

    def test_budget_split(self, planner):
        commits = [self._commit(2000) for _ in range(5)]
        batches = planner.make_batches(commits, budget=2000)
        assert len(batches) > 1

    def test_max_commits_split(self, planner):
        commits = [self._commit(10) for _ in range(10)]
        batches = planner.make_batches(commits, budget=100000, max_commits=3)
        assert all(len(b) <= 3 for b in batches)

    def test_empty_input(self, planner):
        assert planner.make_batches([], budget=1000) == []
