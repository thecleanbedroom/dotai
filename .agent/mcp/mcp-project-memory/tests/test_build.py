"""Tests for BuildAgent — JSON-first two-pass architecture."""

import json
import tempfile
from pathlib import Path

from src.build import BuildAgent
from src.git import GitLogParser
from src.llm.client import LLMClient
from src.llm.openrouter import OpenRouterAPI, RateLimiter
from src.memory import json as json_store
from src.memory.models import Memory


class MockOpenRouterAPI(OpenRouterAPI):
    """Mock OpenRouter API for tests."""

    def __init__(self):
        object.__init__(self)

    def get_model_info(self, model_id: str) -> dict:
        return {
            "context_length": 1_000_000,
            "max_completion_tokens": 65_536,
            "name": "mock-model",
            "supported_parameters": ["structured_outputs"],
            "pricing": {"prompt": 0, "completion": 0},
            "is_free": True,
        }

    def validate_model(self, model_id: str) -> None:
        pass

    def create_rate_limiter(self, model_id: str):
        return RateLimiter(rpm=20)

    def estimate_cost(self, model_id: str, input_tokens: int, output_tokens: int = 0) -> float:
        return 0.0


class MockLLMClient(LLMClient):
    """Mock LLM client that returns pre-defined responses."""

    def __init__(self, response: dict):
        object.__init__(self)
        self._response = response
        self.calls: list[list[dict]] = []
        self.model = "mock-model"

    def chat(self, messages: list[dict], *, temperature: float = 0.2,
             max_tokens: int = 16384,
             response_schema: dict | None = None,
             label: str = "",
             print_lock: object | None = None,
             suppress_stats: bool = False) -> str:
        self.calls.append(messages)
        self.last_usage = {
            "prompt_tokens": 0, "completion_tokens": 0,
            "cached_tokens": 0, "elapsed": 0,
        }
        return json.dumps(self._response)

    def get_model_info(self) -> dict:
        return {
            "context_length": 1_000_000,
            "max_completion_tokens": 65_536,
            "name": "mock-model",
            "pricing": {"prompt": 0, "completion": 0},
            "is_free": True,
        }

    def validate_model(self) -> None:
        pass


# Default synthesis response (accept all, no links/updates)
EMPTY_SYNTHESIS = {
    "accepted_ids": [],
    "rejected_ids": [],
    "update_existing": [],
    "new_links": [],
}


class AcceptAllSynthesis(MockLLMClient):
    """Synthesis mock that handles two-phase synthesis (triage + linking)."""

    def __init__(self):
        super().__init__(EMPTY_SYNTHESIS)

    def chat(self, messages: list[dict], **kwargs) -> str:
        self.calls.append(messages)
        self.last_usage = {
            "prompt_tokens": 0, "completion_tokens": 0,
            "cached_tokens": 0, "elapsed": 0,
        }
        system_content = messages[0]["content"] if messages else ""

        # Phase 1: triage — accept all new memory IDs
        if "triage" in system_content.lower():
            user_content = messages[-1]["content"] if messages else ""
            import re
            ids = re.findall(r'"id":\s*"([^"]+)"', user_content.split("EXISTING corpus")[0])
            return json.dumps({
                "accepted_ids": ids,
                "rejected_ids": [],
            })

        # Phase 2: linking — return empty links
        return json.dumps({"update_existing": [], "new_links": []})


class TestBuildAgent:
    def _make_agent(self, components, extract_response,
                    synthesis_response=None, data_dir=None):
        """Create a BuildAgent with mock LLM clients.

        Args:
            extract_response: Response for Pass 1 (extraction).
            synthesis_response: Response for Pass 2 (synthesis).
                                If None, uses AcceptAllSynthesis.
            data_dir: Temp directory for JSON files.
        """
        extract_llm = MockLLMClient(extract_response)
        if synthesis_response is not None:
            reasoning_llm = MockLLMClient(synthesis_response)
        else:
            reasoning_llm = AcceptAllSynthesis()

        # Create temp data dir if not provided
        if data_dir is None:
            data_dir = Path(tempfile.mkdtemp())

        # Patch the parser to return canned data
        class PatchedParser(GitLogParser):
            def get_all_hashes(self, *, limit=None):
                return ["aaa111"]

            def get_commits_by_hashes(self, hashes):
                raw = "\x00commit aaa111\nAuthor: Dev\nDate: 2026-03-09 10:00:00 -0500\n\nAdd auth service\n\nType: feature\nConfidence: high\n\n src/auth.py | 50 ++++++++++++++++++++\n 1 file changed, 50 insertions(+)"
                return self.parse(raw)

            def get_current_hash(self):
                return "aaa111"

        agent = BuildAgent(
            components["db"],
            components["memory_store"],
            components["link_store"],
            components["build_meta_store"],
            PatchedParser(),
            extract_llm,
            reasoning_llm=reasoning_llm,
            openrouter=MockOpenRouterAPI(),
        )

        # Patch _data_dir to use our temp directory
        agent._data_dir = lambda: data_dir

        return agent, extract_llm, reasoning_llm, data_dir

    def test_build_creates_memories(self, components):
        """Build should create new memories from extraction pass."""
        agent, _, _, data_dir = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "auth_jwt",
                    "summary": "Auth service uses JWT tokens",
                    "type": "decision",
                    "confidence": 75,
                    "importance": 80,
                    "source_commits": ["aaa111"],
                    "files": ["src/auth.py"],
                    "tags": ["auth", "jwt"],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        })
        result = agent.build()
        assert result["status"] == "success"
        assert result["new_memories"] == 1
        assert result["commits_processed"] == 1

        # Verify memory was stored as JSON file
        memories = json_store.read_all_memories(data_dir)
        assert len(memories) == 1
        assert memories[0].summary == "Auth service uses JWT tokens"

        # Verify processed.json was updated
        processed = json_store.read_processed(data_dir)
        assert "aaa111" in processed

    def test_build_records_meta(self, components):
        """Build should record build metadata."""
        agent, _, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "confidence": 40,
                    "importance": 50,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        })
        agent.build()
        last = components["build_meta_store"].get_last()
        assert last is not None
        assert last.build_type == "incremental"

    def test_incremental_build_skips_processed(self, components):
        """Build should skip already-processed commits."""
        data_dir = Path(tempfile.mkdtemp())
        # Pre-populate processed.json with the commit hash
        json_store.add_processed({"aaa111"}, data_dir)
        # Write a memory so self-heal doesn't clear processed.json
        existing = Memory(summary="Existing", type="context", importance=50,
                          source_commits=["aaa111"],
                          created_at="2026-03-09 10:00:00 -0500")
        json_store.write_memory(existing, data_dir)

        agent, _, _, _ = self._make_agent(components, {
            "new_memories": [{"key": "x", "summary": "X", "type": "context",
                              "importance": 50, "source_commits": ["aaa111"],
                              "files": [], "tags": []}],
        }, data_dir=data_dir)
        result = agent.build()
        assert result["status"] == "no_new_commits"
        assert result["commits_processed"] == 0

    def test_synthesis_creates_links(self, components):
        """Synthesis pass should create memory links in JSON files."""
        data_dir = Path(tempfile.mkdtemp())

        # Pre-create two existing memories as JSON files
        m1 = Memory(summary="Memory A", type="decision", importance=80)
        m2 = Memory(summary="Memory B", type="pattern", importance=70)
        json_store.write_memory(m1, data_dir)
        json_store.write_memory(m2, data_dir)

        agent, _, _, _ = self._make_agent(
            components,
            extract_response={
                "new_memories": [
                    {
                        "key": "mem_c",
                        "summary": "Memory C",
                        "type": "context",
                        "importance": 50,
                        "source_commits": ["aaa111"],
                        "files": [],
                        "tags": [],
                        "created_at": "2026-03-09 10:00:00 -0500",
                    }
                ],
            },
            synthesis_response={
                "accepted_ids": [],  # AcceptAllSynthesis handles this
                "rejected_ids": [],
                "update_existing": [],
                "new_links": [
                    {
                        "source": m1.id,
                        "target": m2.id,
                        "relationship": "related_to",
                        "strength": 0.8,
                    }
                ],
            },
            data_dir=data_dir,
        )
        result = agent.build()
        assert result["new_links"] >= 1

    def test_synthesis_no_deactivation(self, components):
        """Synthesis pass should not deactivate — memories persist with metadata."""
        data_dir = Path(tempfile.mkdtemp())

        old = Memory(summary="Still exists after synthesis", type="decision", importance=80)
        json_store.write_memory(old, data_dir)

        agent, _, _, _ = self._make_agent(
            components,
            extract_response={
                "new_memories": [
                    {
                        "key": "replacement",
                        "summary": "Replacement memory",
                        "type": "decision",
                        "importance": 90,
                        "source_commits": ["aaa111"],
                        "files": [],
                        "tags": [],
                        "created_at": "2026-03-09 10:00:00 -0500",
                    }
                ],
            },
            data_dir=data_dir,
        )
        result = agent.build()
        assert "deactivated_memories" not in result

        # Old memory should still exist and be active
        existing = json_store.read_memory(old.id, data_dir)
        assert existing is not None
        assert existing.active is True

    def test_extract_llm_called_with_commits(self, components):
        """Extraction pass should send commits to the LLM."""
        agent, extract_mock, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "importance": 50,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        })
        agent.build()
        assert len(extract_mock.calls) >= 1
        user_msg = extract_mock.calls[0][1]["content"]
        assert "aaa111" in user_msg
        assert "Add auth service" in user_msg

    def test_synthesis_called_after_extraction(self, components):
        """Synthesis pass should run after extraction."""
        agent, _extract_mock, reasoning_mock, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "importance": 50,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        })
        agent.build()
        assert len(reasoning_mock.calls) == 2  # phase 1 (triage) + phase 2 (linking)

    def test_build_result_has_no_decay(self, components):
        """Build result should not include decayed_memories key."""
        agent, _, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "importance": 50,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        })
        result = agent.build()
        assert "decayed_memories" not in result

    def test_reset_wipes_previous(self, components):
        """Reset should wipe existing JSON files and reprocess."""
        data_dir = Path(tempfile.mkdtemp())

        # Pre-populate with existing memory
        old = Memory(summary="Should be gone after reset", type="context", importance=50)
        json_store.write_memory(old, data_dir)
        json_store.add_processed({"old_hash"}, data_dir)

        agent, _, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "fresh_start",
                    "summary": "Fresh start",
                    "type": "context",
                    "importance": 50,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                    "created_at": "2026-03-09 10:00:00 -0500",
                }
            ],
        }, data_dir=data_dir)
        result = agent.reset()
        assert result["status"] == "success"

        # Only the new memory should exist
        memories = json_store.read_all_memories(data_dir)
        assert len(memories) == 1
        assert memories[0].summary == "Fresh start"

        # processed.json should have been reset
        processed = json_store.read_processed(data_dir)
        assert "old_hash" not in processed
        assert "aaa111" in processed
