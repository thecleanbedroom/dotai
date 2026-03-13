"""Synthesis agent — two-phase synthesis pass and result application.

Phase 1 (Triage): Accept/reject new memories, deactivate superseded existing.
Phase 2 (Linking): Find relationships and suggest updates in batched groups.

Extracted from BuildAgent to satisfy Single Responsibility:
BuildAgent orchestrates; SynthesisAgent handles the compare-and-merge logic.
"""

import json
import sys
from dataclasses import dataclass, field
from pathlib import Path

from src.build.retry import call_with_retries
from src.config.internal import InternalSettings
from src.utils import estimate_tokens

from src.llm.client import LLMClient
from src.memory import json as json_store
from src.memory.models import Memory


@dataclass
class SynthesisResult:
    """Structured result from a synthesis pass."""

    accepted_ids: list = field(default_factory=list)
    rejected_ids: list = field(default_factory=list)
    update_existing: list = field(default_factory=list)
    new_links: list = field(default_factory=list)
    error: str | None = None

    def to_dict(self) -> dict:
        """Convert to dict for validation and backward compatibility."""
        d: dict = {
            "accepted_ids": self.accepted_ids,
            "rejected_ids": self.rejected_ids,
            "update_existing": self.update_existing,
            "new_links": self.new_links,
        }
        if self.error is not None:
            d["error"] = self.error
        return d


class SynthesisAgent:
    """Pass 2: Compare new memories against the existing corpus.

    Two-phase approach:
      Phase 1 (Triage) — accept/reject new memories
      Phase 2 (Linking) — find relationships, suggest updates in batched groups
    """

    # ── Serialization helpers ──

    @staticmethod
    def _serialize_full(memories: list[Memory]) -> list[dict]:
        """Full memory dict for LLM prompts."""
        return [
            {"id": m.id, "summary": m.summary, "type": m.type,
             "confidence": m.confidence, "importance": m.importance,
             "file_paths": m.file_paths, "tags": m.tags,
             "source_commits": m.source_commits}
            for m in memories
        ]

    @staticmethod
    def _serialize_compact(memories: list[Memory]) -> list[dict]:
        """Compact summary dict — id/summary/type/tags only."""
        return [
            {"id": m.id, "summary": m.summary, "type": m.type, "tags": m.tags}
            for m in memories
        ]

    @staticmethod
    def _make_batches(
        memories: list[Memory], batch_size: int,
    ) -> list[list[Memory]]:
        """Split memories into batches of the given size."""
        return [
            memories[i:i + batch_size]
            for i in range(0, len(memories), batch_size)
        ]

    # ── Phase 1: Triage ──

    @staticmethod
    def _run_triage(
        llm: LLMClient,
        new_memories: list[Memory],
        existing_data: list[dict],
    ) -> dict | None:
        """Phase 1 — decide which memories to accept/reject.

        Output is pure UUID lists — O(n), always fits in model output.
        """
        new_data = SynthesisAgent._serialize_full(new_memories)
        user_msg = (
            f"NEW memories ({len(new_data)}):\n```json\n"
            + json.dumps(new_data)
            + f"\n```\n\nEXISTING corpus ({len(existing_data)} memories):\n```json\n"
            + (json.dumps(existing_data) if existing_data else "[]")
            + "\n```"
        )

        model_info = llm.get_model_info()
        max_output = model_info.get("max_completion_tokens", 65_536)

        return call_with_retries(
            llm,
            [
                {"role": "system", "content": InternalSettings.synthesis_triage_prompt()},
                {"role": "user", "content": user_msg},
            ],
            max_tokens=max_output,
            response_schema=InternalSettings.synthesis_triage_schema(),
        )

    # ── Phase 2: Linking ──

    @staticmethod
    def _compute_linking_batch_size(
        llm: LLMClient,
        memories: list[Memory],
        corpus_data: list[dict],
    ) -> int:
        """Compute linking batch size dynamically from model context.

        Fixed cost per batch: corpus JSON + system prompt + output reserve.
        Variable cost: each batch memory (full serialization).
        Fills to ~80% of remaining context after fixed costs.
        """
        model_info = llm.get_model_info()
        context = model_info.get("context_length", 1_000_000)
        max_output = model_info.get("max_completion_tokens", 65_536)

        # Fixed costs
        system_prompt_tokens = estimate_tokens(
            InternalSettings.synthesis_linking_prompt()
        )
        corpus_tokens = estimate_tokens(json.dumps(corpus_data)) if corpus_data else 0
        fixed_overhead = system_prompt_tokens + corpus_tokens + max_output

        available = int((context - fixed_overhead) * InternalSettings.synthesis_context_fill_ratio())
        if available <= 0 or not memories:
            return max(len(memories), 10)

        # Estimate per-memory cost from a sample
        sample = memories[:min(5, len(memories))]
        sample_data = SynthesisAgent._serialize_full(sample)
        per_memory_tokens = estimate_tokens(json.dumps(sample_data)) // len(sample)

        if per_memory_tokens <= 0:
            return max(len(memories), 10)

        batch_size = available // per_memory_tokens
        batch_size = max(batch_size, 20)    # floor — too small = too many round trips

        return batch_size

    @staticmethod
    def _run_linking_batch(
        llm: LLMClient,
        batch: list[Memory],
        corpus_data: list[dict],
    ) -> dict | None:
        """Phase 2 — find links and suggest updates for one batch.

        Each batch is compared against the full corpus.
        Batch size is dynamically computed in run() to fill context.
        """
        batch_data = SynthesisAgent._serialize_full(batch)

        user_msg = (
            f"BATCH memories ({len(batch_data)}):\n```json\n"
            + json.dumps(batch_data)
            + f"\n```\n\nCORPUS ({len(corpus_data)} memories):\n```json\n"
            + (json.dumps(corpus_data) if corpus_data else "[]")
            + "\n```"
        )

        model_info = llm.get_model_info()
        max_output = model_info.get("max_completion_tokens", 65_536)

        return call_with_retries(
            llm,
            [
                {"role": "system", "content": InternalSettings.synthesis_linking_prompt()},
                {"role": "user", "content": user_msg},
            ],
            max_tokens=max_output,
            response_schema=InternalSettings.synthesis_linking_schema(),
        )

    # ── Run (two-phase orchestrator) ──

    def run(
        self, llm: LLMClient,
        new_memories: list[Memory],
        existing_memories: list[Memory],
    ) -> SynthesisResult:
        """Two-phase synthesis — triage then link.

        Phase 1: Accept/reject all new memories.
        Phase 2: Link accepted memories to the full corpus in batched groups.
        """
        if not new_memories:
            return SynthesisResult()

        result = SynthesisResult()
        errors: list[str] = []

        # Pre-compute existing corpus (compact) — used by both phases
        existing_data = self._serialize_compact(existing_memories) if existing_memories else []

        # ── Phase 1: Triage ──
        print(
            f"    phase 1: triaging {len(new_memories)} memories...",
            file=sys.stderr, flush=True,
        )
        triage_result = self._run_triage(llm, new_memories, existing_data)

        if triage_result and "error" not in triage_result:
            result.accepted_ids = triage_result.get("accepted_ids", [])
            result.rejected_ids = triage_result.get("rejected_ids", [])
        elif triage_result and "error" in triage_result:
            errors.append(f"triage: {triage_result['error']}")
            # On triage failure, accept everything so linking can still run
            result.accepted_ids = [m.id for m in new_memories]

        # Unmentioned memories are accepted by default
        mentioned = set(result.accepted_ids) | set(result.rejected_ids)
        unmentioned = [m.id for m in new_memories if m.id not in mentioned]
        result.accepted_ids.extend(unmentioned)

        accepted_set = set(result.accepted_ids)
        accepted_memories = [m for m in new_memories if m.id in accepted_set]

        rejected_count = len(result.rejected_ids)
        print(
            f"    phase 1: {len(accepted_memories)} accepted, {rejected_count} rejected",
            file=sys.stderr, flush=True,
        )

        # ── Phase 2: Linking ──
        if not accepted_memories:
            if errors:
                result.error = "; ".join(errors)
            return result

        # Corpus for linking = existing + accepted new (compact)
        accepted_compact = self._serialize_compact(accepted_memories)
        full_corpus = existing_data + accepted_compact

        batch_size = self._compute_linking_batch_size(
            llm, accepted_memories, full_corpus,
        )
        batches = self._make_batches(accepted_memories, batch_size)

        print(
            f"    phase 2: linking {len(accepted_memories)} memories in {len(batches)} batch(es)...",
            file=sys.stderr, flush=True,
        )

        for batch_idx, batch in enumerate(batches):
            if len(batches) > 1:
                print(
                    f"    linking batch [{batch_idx + 1}/{len(batches)}] "
                    f"({len(batch)} memories)...",
                    file=sys.stderr, flush=True,
                )

            link_result = self._run_linking_batch(llm, batch, full_corpus)

            if link_result and "error" not in link_result:
                result.update_existing.extend(link_result.get("update_existing", []))
                result.new_links.extend(link_result.get("new_links", []))
            elif link_result and "error" in link_result:
                errors.append(f"linking batch {batch_idx + 1}: {link_result['error']}")

        if errors:
            result.error = "; ".join(errors)

        return result

    # ── Apply results — sub-methods ──

    @staticmethod
    def _apply_rejections(
        result: SynthesisResult, new_memories: list[Memory], data_dir: Path,
    ) -> int:
        """Mark rejected memories as inactive and return count."""
        rejected_ids = set(result.rejected_ids)
        count = 0
        for mem in new_memories:
            if mem.id in rejected_ids:
                mem.active = False
                json_store.update_memory(mem, data_dir)
                count += 1
        return count


    @staticmethod
    def _apply_updates(result: SynthesisResult, data_dir: Path) -> int:
        """Apply importance/summary adjustments to existing memories."""
        count = 0
        for update in result.update_existing:
            uid = update.get("id", "")
            existing = json_store.read_memory(uid, data_dir)
            if existing:
                if "summary" in update:
                    existing.summary = update["summary"]
                if "importance" in update:
                    existing.importance = update["importance"]
                json_store.update_memory(existing, data_dir)
                count += 1
        return count

    @staticmethod
    def _apply_links(result: SynthesisResult, data_dir: Path) -> int:
        """Embed links in source memory JSON files. Returns link count."""
        # Group links by source memory
        links_by_source: dict[str, list[dict]] = {}
        for link_data in result.new_links:
            source = link_data.get("source", link_data.get("memory_id_a", ""))
            if source:
                links_by_source.setdefault(source, []).append(link_data)

        # Embed links into each source memory's JSON file
        for source_id, links in links_by_source.items():
            mem = json_store.read_memory(source_id, data_dir)
            if mem:
                existing_keys = {
                    (l.get("target", l.get("memory_id_b", "")), l.get("relationship", ""))
                    for l in mem.links
                }
                for link in links:
                    key = (
                        link.get("target", link.get("memory_id_b", "")),
                        link.get("relationship", ""),
                    )
                    if key not in existing_keys:
                        mem.links.append(link)
                        existing_keys.add(key)
                json_store.update_memory(mem, data_dir)

        return len(result.new_links)

    # ── Apply results — orchestrator ──

    @staticmethod
    def apply_results(
        synth_result: SynthesisResult,
        new_memories: list[Memory],
        data_dir: Path,
        *,
        links_only: bool = False,
    ) -> dict:
        """Apply synthesis results to memory JSON files on disk.

        When links_only=True, only apply links and updates — skip
        rejections.  Used by synthesis-only re-runs where all memories
        are already accepted.

        Returns counts: rejected, updated, links.
        """
        rejected_count = 0

        if not links_only:
            rejected_count = SynthesisAgent._apply_rejections(synth_result, new_memories, data_dir)

        updated_count = SynthesisAgent._apply_updates(synth_result, data_dir)
        link_count = SynthesisAgent._apply_links(synth_result, data_dir)

        return {
            "rejected": rejected_count,
            "updated": updated_count,
            "links": link_count,
        }
