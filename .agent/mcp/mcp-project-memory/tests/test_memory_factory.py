"""Tests for MemoryFactory — confidence scoring and Memory construction."""

from src.build.memory_factory import MemoryFactory
from src.memory.models import Memory


class TestFromLlmOutput:
    """Test MemoryFactory.from_llm_output confidence scoring."""

    def test_minimal_input(self):
        data = {
            "summary": "short",
            "type": "context",
            "importance": 30,
            "source_commits": [],
            "files": [],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert isinstance(mem, Memory)
        assert mem.confidence == 0  # all zero signals
        assert mem.summary == "short"
        assert mem.type == "context"
        assert mem.importance == 30

    def test_single_commit_score(self):
        data = {
            "summary": "x",
            "source_commits": ["abc123"],
            "files": [],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 8  # 1 commit → 8

    def test_two_commits_score(self):
        data = {
            "summary": "x",
            "source_commits": ["a", "b"],
            "files": [],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 20  # 2 commits → 20

    def test_three_plus_commits_score(self):
        data = {
            "summary": "x",
            "source_commits": ["a", "b", "c"],
            "files": [],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 30  # 3+ commits → 30

    def test_file_scoring_one(self):
        data = {
            "summary": "x",
            "source_commits": [],
            "files": ["a.py"],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 5  # 1 file → 5

    def test_file_scoring_two(self):
        data = {
            "summary": "x",
            "source_commits": [],
            "files": ["a.py", "b.py"],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 15  # 2-3 files → 15

    def test_file_scoring_four(self):
        data = {
            "summary": "x",
            "source_commits": [],
            "files": ["a", "b", "c", "d"],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 25  # 4-6 files → 25

    def test_file_scoring_seven(self):
        data = {
            "summary": "x",
            "source_commits": [],
            "files": [f"f{i}" for i in range(7)],
            "tags": [],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 30  # 7+ files → 30

    def test_summary_length_scoring(self):
        # ≤100 chars → 0
        data = {"summary": "x" * 50, "source_commits": [], "files": [], "tags": []}
        assert MemoryFactory.from_llm_output(data).confidence == 0

        # 101-200 → 5
        data["summary"] = "x" * 150
        assert MemoryFactory.from_llm_output(data).confidence == 5

        # 201-300 → 12
        data["summary"] = "x" * 250
        assert MemoryFactory.from_llm_output(data).confidence == 12

        # 301+ → 20
        data["summary"] = "x" * 350
        assert MemoryFactory.from_llm_output(data).confidence == 20

    def test_tag_scoring(self):
        # ≤2 tags → 0
        data = {"summary": "x", "source_commits": [], "files": [], "tags": ["a", "b"]}
        assert MemoryFactory.from_llm_output(data).confidence == 0

        # 3-4 → 5
        data["tags"] = ["a", "b", "c"]
        assert MemoryFactory.from_llm_output(data).confidence == 5

        # 5-6 → 12
        data["tags"] = ["a", "b", "c", "d", "e"]
        assert MemoryFactory.from_llm_output(data).confidence == 12

        # 7+ → 20
        data["tags"] = [f"t{i}" for i in range(8)]
        assert MemoryFactory.from_llm_output(data).confidence == 20

    def test_max_score(self):
        """Full signals: 30 (commits) + 30 (files) + 20 (summary) + 20 (tags) = 100."""
        data = {
            "summary": "x" * 350,
            "source_commits": ["a", "b", "c"],
            "files": [f"f{i}" for i in range(7)],
            "tags": [f"t{i}" for i in range(8)],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.confidence == 100

    def test_defaults_for_missing_fields(self):
        data = {}  # empty dict
        mem = MemoryFactory.from_llm_output(data)
        assert mem.type == "context"
        assert mem.importance == 50
        assert mem.source_commits == []
        assert mem.file_paths == []
        assert mem.tags == []

    def test_fields_passed_through(self):
        data = {
            "summary": "test summary",
            "type": "decision",
            "importance": 90,
            "source_commits": ["abc"],
            "files": ["foo.py"],
            "tags": ["python"],
        }
        mem = MemoryFactory.from_llm_output(data)
        assert mem.summary == "test summary"
        assert mem.type == "decision"
        assert mem.importance == 90
        assert mem.source_commits == ["abc"]
        assert mem.file_paths == ["foo.py"]
        assert mem.tags == ["python"]
