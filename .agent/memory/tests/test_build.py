"""Tests for BuildAgent with mock LLM — two-pass architecture."""

import json

from src.models import Memory, MemoryLink
from src.git import GitLogParser
from src.build import BuildAgent


class MockLLMClient:
    """Mock LLM client that returns pre-defined responses."""

    def __init__(self, response: dict):
        self._response = response
        self.calls = []

    def chat(self, messages: list[dict], *, temperature: float = 0.2,
             max_tokens: int = 16384,
             response_schema: dict | None = None) -> str:
        self.calls.append(messages)
        return json.dumps(self._response)

    def get_model_info(self) -> dict:
        return {
            "context_length": 1_000_000,
            "max_completion_tokens": 65_536,
            "name": "mock-model",
        }

    def validate_model(self) -> None:
        pass  # Always valid in tests


# Default synthesis response (no links, no updates, no deactivations)
EMPTY_SYNTHESIS = {
    "update_memories": [],
    "deactivate_memory_ids": [],
    "new_links": [],
}


class TestBuildAgent:
    def _make_agent(self, components, extract_response,
                    synthesis_response=None):
        """Create a BuildAgent with mock LLM clients.

        Args:
            extract_response: Response for Pass 1 (extraction).
            synthesis_response: Response for Pass 2 (synthesis).
                                Defaults to empty (no links/updates).
        """
        extract_llm = MockLLMClient(extract_response)
        reasoning_llm = MockLLMClient(
            synthesis_response or EMPTY_SYNTHESIS
        )

        # Patch the parser to return canned data
        class PatchedParser(GitLogParser):
            def get_file_list(self, *, since_hash=None, limit=None):
                return """commit aaa111
Author: Dev
Date: 2026-03-09 10:00:00 -0500

Add auth service

Type: feature
Confidence: high

 src/auth.py | 50 ++++++++++++++++++++
 1 file changed, 50 insertions(+)

---END_COMMIT---"""

            def get_current_hash(self):
                return "aaa111"

        return BuildAgent(
            components["db"],
            components["memory_store"],
            components["link_store"],
            components["build_meta_store"],
            PatchedParser(),
            extract_llm,
            reasoning_llm=reasoning_llm,
        ), extract_llm, reasoning_llm

    def test_build_creates_memories(self, components):
        """Build should create new memories from extraction pass."""
        agent, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "auth_jwt",
                    "summary": "Auth service uses JWT tokens",
                    "type": "decision",
                    "confidence": "high",
                    "importance": 0.8,
                    "source_commits": ["aaa111"],
                    "files": ["src/auth.py"],
                    "tags": ["auth", "jwt"],
                }
            ],
        })
        result = agent.build()
        assert result["status"] == "success"
        assert result["new_memories"] == 1
        assert result["commits_processed"] == 1

        # Verify memory was stored
        memories = components["memory_store"].list_all()
        assert len(memories) == 1
        assert memories[0].summary == "Auth service uses JWT tokens"

    def test_build_records_meta(self, components):
        """Build should record build metadata."""
        agent, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        agent.build()
        last = components["build_meta_store"].get_last()
        assert last is not None
        assert last.last_commit == "aaa111"
        assert last.build_type == "full"

    def test_synthesis_creates_links(self, components):
        """Synthesis pass should create memory links."""
        # Pre-create two memories so the synthesis pass can link them
        m1 = Memory(
            summary="Memory A", type="decision", confidence="high",
            importance=0.8, source_commits=["bbb222"], files=["src/a.py"],
        )
        m2 = Memory(
            summary="Memory B", type="pattern", confidence="high",
            importance=0.7, source_commits=["bbb222"], files=["src/b.py"],
        )
        m1 = components["memory_store"].create(m1)
        m2 = components["memory_store"].create(m2)

        agent, _, _ = self._make_agent(
            components,
            extract_response={
                "new_memories": [
                    {
                        "key": "mem_c",
                        "summary": "Memory C",
                        "type": "context",
                        "confidence": "medium",
                        "importance": 0.5,
                        "source_commits": ["aaa111"],
                        "files": [],
                        "tags": [],
                    }
                ],
            },
            synthesis_response={
                "update_memories": [],
                "deactivate_memory_ids": [],
                "new_links": [
                    {
                        "source": m1.id,
                        "target": m2.id,
                        "relationship": "related_to",
                        "strength": 0.8,
                    }
                ],
            },
        )
        result = agent.build()
        assert result["new_links"] == 1

        # Verify the link exists
        links = components["link_store"].get_links_for(m1.id)
        assert len(links) == 1

    def test_synthesis_deactivates_memories(self, components):
        """Synthesis pass should deactivate memories."""
        # Create a memory to be deactivated
        old = Memory(
            summary="Will be deactivated", type="decision",
            confidence="high", importance=0.8,
            source_commits=["bbb222"], files=[],
        )
        old = components["memory_store"].create(old)
        old_id = old.id

        agent, _, _ = self._make_agent(
            components,
            extract_response={
                "new_memories": [
                    {
                        "key": "replacement",
                        "summary": "Replacement memory",
                        "type": "decision",
                        "confidence": "high",
                        "importance": 0.9,
                        "source_commits": ["aaa111"],
                        "files": [],
                        "tags": [],
                    }
                ],
            },
            synthesis_response={
                "update_memories": [],
                "deactivate_memory_ids": [old_id],
                "new_links": [],
            },
        )
        result = agent.build()
        assert result["deactivated_memories"] == 1
        fetched = components["memory_store"].get(old_id)
        assert fetched.active is False

    def test_build_always_does_full_rebuild(self, components):
        """Rebuild should always drop and recreate (full rebuild)."""
        # First build
        agent, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "to_be_gone",
                    "summary": "Should be gone after rebuild",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        result = agent.build()
        assert result["new_memories"] == 1

        # Rebuild — old memory should be gone
        agent2, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "fresh_start",
                    "summary": "Fresh start",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        result2 = agent2.rebuild()
        assert result2["status"] == "success"
        memories = components["memory_store"].list_all(active_only=False)
        assert len(memories) == 1
        assert memories[0].summary == "Fresh start"

    def test_extract_llm_called_with_commits(self, components):
        """Extraction pass should send commits to the LLM."""
        agent, extract_mock, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        agent.build()
        # Extract LLM should be called for batch processing
        assert len(extract_mock.calls) >= 1
        user_msg = extract_mock.calls[0][1]["content"]
        assert "aaa111" in user_msg
        assert "Add auth service" in user_msg

    def test_synthesis_called_after_extraction(self, components):
        """Synthesis pass should run after extraction."""
        agent, extract_mock, reasoning_mock = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        agent.build()
        # Reasoning LLM should be called for synthesis
        assert len(reasoning_mock.calls) == 1

    def test_build_result_has_no_decay(self, components):
        """Build result should not include decayed_memories key."""
        agent, _, _ = self._make_agent(components, {
            "new_memories": [
                {
                    "key": "placeholder",
                    "summary": "Placeholder",
                    "type": "context",
                    "confidence": "medium",
                    "importance": 0.5,
                    "source_commits": ["aaa111"],
                    "files": [],
                    "tags": [],
                }
            ],
        })
        result = agent.build()
        assert "decayed_memories" not in result

    def test_synthesis_auto_deactivates_superseded(self, components):
        """Supersedes links should auto-deactivate the target memory."""
        # Pre-create a memory that will be superseded
        old = Memory(
            summary="Old approach", type="decision", confidence="high",
            importance=0.8, source_commits=["bbb222"], files=["src/old.py"],
        )
        old = components["memory_store"].create(old)
        old_id = old.id

        agent, _, _ = self._make_agent(
            components,
            extract_response={
                "new_memories": [
                    {
                        "key": "new_approach",
                        "summary": "New approach replaces old",
                        "type": "decision",
                        "confidence": "high",
                        "importance": 0.9,
                        "source_commits": ["aaa111"],
                        "files": ["src/new.py"],
                        "tags": ["refactor"],
                    },
                ],
            },
            synthesis_response={
                "update_memories": [],
                "deactivate_memory_ids": [],
                "new_links": [
                    {
                        "source": 999,  # Placeholder — new memory ID unknown
                        "target": old_id,
                        "relationship": "supersedes",
                        "strength": 0.9,
                    }
                ],
            },
        )
        result = agent.build()
        assert result["new_memories"] == 1
        # Supersedes auto-deactivation happens when link is created
        assert result["deactivated_memories"] >= 1

        fetched = components["memory_store"].get(old_id)
        assert fetched.active is False
