"""Tests for SynthesisAgent — two-phase synthesis (triage + linking) and result application."""

import json
from unittest.mock import MagicMock, patch

import pytest

from src.build.synthesis import SynthesisAgent, SynthesisResult
from src.memory.models import Memory


def _make_memory(**overrides) -> Memory:
    """Create a Memory with sensible defaults."""
    defaults = {
        "summary": "test memory summary",
        "type": "decision",
        "confidence": 50,
        "importance": 50,
        "tags": ["test"],
    }
    defaults.update(overrides)
    return Memory(**defaults)


@pytest.fixture()
def agent():
    return SynthesisAgent()


# -- Serialization helpers --

class TestSerializeHelpers:
    def test_serialize_full_includes_all_fields(self):
        m = _make_memory(summary="test", importance=75)
        result = SynthesisAgent._serialize_full([m])
        assert len(result) == 1
        assert result[0]["summary"] == "test"
        assert result[0]["importance"] == 75
        assert "source_commits" in result[0]

    def test_serialize_compact_limited_fields(self):
        m = _make_memory(summary="test", importance=75)
        result = SynthesisAgent._serialize_compact([m])
        assert len(result) == 1
        assert result[0]["summary"] == "test"
        assert "importance" not in result[0]


# -- _make_batches --

class TestMakeBatches:
    def test_exact_division(self):
        memories = [_make_memory() for _ in range(6)]
        batches = SynthesisAgent._make_batches(memories, 3)
        assert len(batches) == 2
        assert all(len(b) == 3 for b in batches)

    def test_remainder(self):
        memories = [_make_memory() for _ in range(7)]
        batches = SynthesisAgent._make_batches(memories, 3)
        assert len(batches) == 3
        assert len(batches[2]) == 1

    def test_single_batch(self):
        memories = [_make_memory() for _ in range(3)]
        batches = SynthesisAgent._make_batches(memories, 10)
        assert len(batches) == 1
        assert len(batches[0]) == 3


# -- SynthesisResult --

class TestSynthesisResult:
    def test_default_empty(self):
        r = SynthesisResult()
        assert r.accepted_ids == []
        assert r.error is None

    def test_to_dict_without_error(self):
        r = SynthesisResult(accepted_ids=["a"], rejected_ids=["b"])
        d = r.to_dict()
        assert d["accepted_ids"] == ["a"]
        assert "error" not in d

    def test_to_dict_with_error(self):
        r = SynthesisResult(error="something failed")
        d = r.to_dict()
        assert d["error"] == "something failed"


# -- apply_results --

class TestApplyResults:
    def test_rejects_mark_inactive(self, agent, tmp_path):
        m1 = _make_memory()
        m2 = _make_memory()

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
            rejected_ids=[m2.id],
        )

        with patch("src.build.synthesis.json_store") as mock_js:
            counts = SynthesisAgent.apply_results(synth_result, [m1, m2], tmp_path)

        assert counts["rejected"] == 1
        assert m2.active is False
        mock_js.update_memory.assert_any_call(m2, tmp_path)

    def test_unmentioned_memories_not_rejected(self, agent, tmp_path):
        m1 = _make_memory()
        m2 = _make_memory()

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
        )

        with patch("src.build.synthesis.json_store"):
            counts = SynthesisAgent.apply_results(synth_result, [m1, m2], tmp_path)

        assert counts["rejected"] == 0


    def test_updates_existing(self, agent, tmp_path):
        m1 = _make_memory()
        existing = _make_memory(summary="old summary", importance=30)

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
            update_existing=[
                {"id": existing.id, "summary": "new summary", "importance": 80}
            ],
        )

        with patch("src.build.synthesis.json_store") as mock_js:
            mock_js.read_memory.return_value = existing
            counts = SynthesisAgent.apply_results(synth_result, [m1], tmp_path)

        assert counts["updated"] == 1
        assert existing.summary == "new summary"
        assert existing.importance == 80

    def test_creates_links(self, agent, tmp_path):
        m1 = _make_memory()
        source = _make_memory()

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
            new_links=[
                {"source": source.id, "target": m1.id,
                 "relationship": "related_to", "strength": 70}
            ],
        )

        with patch("src.build.synthesis.json_store") as mock_js:
            mock_js.read_memory.return_value = source
            mock_js.delete_memory.return_value = False
            counts = SynthesisAgent.apply_results(synth_result, [m1], tmp_path)

        assert counts["links"] == 1

    def test_supersedes_link_is_just_a_link(self, agent, tmp_path):
        m1 = _make_memory()
        source = _make_memory()

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
            new_links=[
                {"source": source.id, "target": "old-target",
                 "relationship": "supersedes", "strength": 90}
            ],
        )

        with patch("src.build.synthesis.json_store") as mock_js:
            mock_js.read_memory.return_value = source
            counts = SynthesisAgent.apply_results(synth_result, [m1], tmp_path)

        assert counts["links"] == 1

    def test_links_only_skips_rejections(self, agent, tmp_path):
        m1 = _make_memory()

        synth_result = SynthesisResult(
            accepted_ids=[m1.id],
            rejected_ids=["should-not-delete"],
            new_links=[
                {"source": m1.id, "target": "other",
                 "relationship": "related_to", "strength": 50}
            ],
        )

        with patch("src.build.synthesis.json_store") as mock_js:
            mock_js.delete_memory.return_value = False
            counts = SynthesisAgent.apply_results(
                synth_result, [m1], tmp_path, links_only=True,
            )

        assert counts["rejected"] == 0
        assert counts["links"] == 1


# -- run (two-phase integration) --

class TestRun:
    def test_empty_input_returns_empty(self, agent):
        result = agent.run(MagicMock(), [], [])
        assert isinstance(result, SynthesisResult)
        assert result.accepted_ids == []

    def test_triage_then_linking(self, agent):
        llm = MagicMock()
        llm.get_model_info.return_value = {
            "context_length": 131_072,
            "max_completion_tokens": 65_536,
        }
        m1 = _make_memory()
        m2 = _make_memory()

        with patch("src.build.synthesis.call_with_retries") as mock_retry:
            # Phase 1: triage response
            triage_response = {
                "accepted_ids": [m1.id],
                "rejected_ids": [m2.id],
                "deactivate_existing": [],
            }
            # Phase 2: linking response
            linking_response = {
                "update_existing": [],
                "new_links": [
                    {"source": m1.id, "target": "existing-1",
                     "relationship": "related_to", "strength": 80}
                ],
            }
            mock_retry.side_effect = [triage_response, linking_response]

            result = agent.run(llm, [m1, m2], [])

        assert isinstance(result, SynthesisResult)
        assert m1.id in result.accepted_ids
        assert m2.id in result.rejected_ids
        assert len(result.new_links) == 1
        assert result.error is None

    def test_triage_failure_accepts_all(self, agent):
        llm = MagicMock()
        llm.get_model_info.return_value = {
            "context_length": 131_072,
            "max_completion_tokens": 65_536,
        }
        m1 = _make_memory()

        with patch("src.build.synthesis.call_with_retries") as mock_retry:
            # Phase 1: triage error
            triage_error = {"error": "triage failed"}
            # Phase 2: linking still runs
            linking_response = {
                "update_existing": [],
                "new_links": [],
            }
            mock_retry.side_effect = [triage_error, linking_response]

            result = agent.run(llm, [m1], [])

        # All accepted on triage failure
        assert m1.id in result.accepted_ids
        assert result.error is not None  # error recorded
        assert "triage" in result.error

    def test_linking_batches_multiple(self, agent):
        llm = MagicMock()
        llm.get_model_info.return_value = {
            "context_length": 131_072,
            "max_completion_tokens": 65_536,
        }
        # Create 20 memories — should produce 2 linking batches with batch_size=10
        memories = [_make_memory() for _ in range(20)]
        ids = [m.id for m in memories]

        with patch("src.build.synthesis.call_with_retries") as mock_retry, \
             patch.object(SynthesisAgent, "_compute_linking_batch_size", return_value=10):
            # Phase 1: accept all
            triage_response = {
                "accepted_ids": ids,
                "rejected_ids": [],
            }
            # Phase 2: two linking batches
            linking_batch_1 = {
                "update_existing": [],
                "new_links": [{"source": ids[0], "target": ids[1],
                               "relationship": "related_to", "strength": 60}],
            }
            linking_batch_2 = {
                "update_existing": [],
                "new_links": [{"source": ids[10], "target": ids[11],
                               "relationship": "related_to", "strength": 70}],
            }
            mock_retry.side_effect = [triage_response, linking_batch_1, linking_batch_2]

            result = agent.run(llm, memories, [])

        # 3 calls total: 1 triage + 2 linking batches
        assert mock_retry.call_count == 3
        assert len(result.new_links) == 2
        assert result.error is None
