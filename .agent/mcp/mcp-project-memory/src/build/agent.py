"""Build agent — orchestrates commit processing, LLM calls, and memory creation.

JSON-first architecture: memories are persisted as individual JSON files
in data/memories/.  The SQLite DB is a disposable runtime cache rebuilt
from those files.  processed.json tracks which commits have been handled.

This module is now a thin orchestrator. Heavy logic lives in:
  build.batching      — token estimation, batch planning
  build.retry         — LLM call retry with fallback escalation
  build.memory_factory — Memory creation with confidence scoring
  build.synthesis     — incremental synthesis pass
"""

import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from src.config.settings import Settings

from src.build.batching import BatchPlanner
from src.build.memory_factory import MemoryFactory
from src.build.retry import call_with_retries
from src.build.synthesis import SynthesisAgent, SynthesisResult
from src.config.internal import InternalSettings
from src.db import Database
from src.git import GitLogParser
from src.llm.client import LLMClient
from src.llm.openrouter import OpenRouterAPI
from src.memory import json as json_store
from src.memory.models import BuildMetaEntry, Memory
from src.memory.stores import BuildMetaStore, LinkStore, MemoryStore
from src.path_filter import PathFilter
from src.build.validators import LLMOutputError, validate_extraction, validate_triage, validate_linking


class BuildAgent:
    """Orchestrates build: parse commits → LLM extraction → synthesis → JSON files.

    Two-pass architecture:
      Pass 1 (extraction): Fast model extracts memories from commit batches.
      Pass 2 (synthesis):  Reasoning model compares new vs existing, links related.

    Delegates heavy logic to:
      BatchPlanner   — sizing, splitting, formatting
      call_with_retries — resilient LLM calls
      MemoryFactory  — Memory construction + confidence scoring
      SynthesisAgent — synthesis pass + result application
    """

    def __init__(
        self,
        db: Database,
        memory_store: MemoryStore,
        link_store: LinkStore,
        build_meta_store: BuildMetaStore,
        git_parser: GitLogParser,
        llm_client: LLMClient,
        config: Optional["Settings"] = None,
        *,
        extract_fallback_llm: LLMClient | None = None,
        reasoning_llm: LLMClient | None = None,
        openrouter: OpenRouterAPI | None = None,
    ):
        if config is None:
            from src.config.settings import Settings
            config = Settings.load()
        self._config = config
        self._openrouter = openrouter or OpenRouterAPI(config)
        self._db = db
        self._memories = memory_store
        self._links = link_store
        self._build_meta = build_meta_store
        self._git = git_parser
        self._llm = llm_client  # fast extraction model (primary)
        self._extract_fallback = extract_fallback_llm  # extraction fallback (cheap)
        self._reasoning_llm = reasoning_llm  # reasoning model for synthesis

        # Delegates
        self._batch_planner = BatchPlanner(config)
        self._synthesis = SynthesisAgent()

    def _data_dir(self) -> Path:
        """Resolve the data directory for this project."""
        from src.utils import data_dir
        return data_dir()

    @staticmethod
    def _log_batch_progress(
        n_done: int,
        total_extracted: int,
        mem_count: int,
        n_commits: int,
        n_total: int,
        build_start: float,
        usage: dict,
    ) -> None:
        """Log progress for a completed batch. Called under print_lock."""
        elapsed = time.time() - build_start
        actual_rpm = (n_done / elapsed * 60) if elapsed > 0 else 0

        remaining = n_total - n_done
        eta_s = (remaining / actual_rpm * 60) if actual_rpm > 0 else 0
        eta_str = f"{int(eta_s)}s" if eta_s < 120 else f"{int(eta_s/60)}m"

        tok_in = usage.get('prompt_tokens', 0)
        tok_out = usage.get('completion_tokens', 0)
        tok_cached = usage.get('cached_tokens', 0)
        call_time = usage.get('elapsed', 0)

        print(
            f"  [{n_done}/{n_total}] "
            f"({n_commits} commits): {call_time}s | "
            f"in: {tok_in:,} (cached: {tok_cached:,}) | "
            f"out: {tok_out:,} | "
            f"+{mem_count} memories | "
            f"ETA {eta_str}",
            file=sys.stderr, flush=True,
        )

    def build(self, *, limit: int | None = None, auto_confirm: bool = False,
              synthesis: bool = False) -> dict:
        """Incremental build — process commits not yet in processed.json.

        Safe to interrupt: processed.json is only updated AFTER successful
        extraction and synthesis, so the next build retries any incomplete work.

        Args:
            limit: Max commits to process (newest first). Reads from
                   MEMORY_COMMIT_LIMIT env var if not provided.
                   0 or None = all new commits.
            auto_confirm: Skip cost confirmation prompt (--yes flag).
            synthesis: Force synthesis pass on all existing memories.
        """
        limit = limit or self._config.batching_commit_limit() or None
        return self._run_build(
            limit=limit,
            build_type="incremental",
            auto_confirm=auto_confirm,
            synthesis=synthesis,
        )

    def reset(self, *, limit: int | None = None, auto_confirm: bool = False) -> dict:
        """Full reset — wipe all JSON files and reprocess entire git history.

        Args:
            limit: Max commits to process (newest first).
                   0 or None = all commits.
            auto_confirm: Skip cost confirmation prompt (--yes flag).
        """
        limit = limit or self._config.batching_commit_limit() or None
        data_dir = self._data_dir()

        # Wipe existing memories and processed state
        memories_dir = data_dir / "memories"
        if memories_dir.exists():
            for f in memories_dir.glob("*.json"):
                f.unlink()
        processed_path = data_dir / "processed.json"
        if processed_path.exists():
            processed_path.unlink()

        # Wipe the DB
        self._db.hold()
        self._db.drop_all()
        self._db.init_schema()
        self._db.release()

        return self._run_build(
            limit=limit,
            build_type="full",
            auto_confirm=auto_confirm,
        )

    def _run_build(
        self,
        *,
        limit: int | None,
        build_type: str,
        auto_confirm: bool = False,
        synthesis: bool = False,
    ) -> dict:
        """Core build logic — two-pass architecture with JSON persistence.

        Pass 1 (extraction): Fast model processes commit batches → raw memories.
        Pass 2 (synthesis): Reasoning model compares new vs existing → accept/reject/link.
        Then: persist accepted memories as JSON, delete superseded, rebuild DB.
        """
        data_dir = self._data_dir()

        # Prevent concurrent builds from corrupting the data directory
        import fcntl
        lock_path = data_dir / "build.lock"
        lock_path.parent.mkdir(parents=True, exist_ok=True)
        lock_file = open(lock_path, "w")
        try:
            fcntl.flock(lock_file, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            lock_file.close()
            raise RuntimeError(
                f"Another build process is already running (lockfile: {lock_path}). "
                f"Wait for it to finish or remove the lockfile if stale."
            )

        try:
            return self._run_build_locked(
                data_dir=data_dir,
                limit=limit,
                build_type=build_type,
                auto_confirm=auto_confirm,
                synthesis=synthesis,
            )
        finally:
            fcntl.flock(lock_file, fcntl.LOCK_UN)
            lock_file.close()

    def _run_build_locked(
        self,
        *,
        data_dir: Path,
        limit: int | None,
        build_type: str,
        auto_confirm: bool = False,
        synthesis: bool = False,
    ) -> dict:
        """Build logic under lock — extracted from _run_build for clarity."""

        # Clear previous build response logs
        responses_dir = data_dir / "build"
        if responses_dir.is_dir():
            for f in responses_dir.iterdir():
                if f.is_file():
                    f.unlink()

        # Phase 1: Validate model & compute budget
        token_budget, max_output = self._validate_and_log_models()

        # Phase 2: Discover unprocessed commits
        commits, _all_hashes = self._discover_commits(
            data_dir, limit, synthesis,
        )
        if commits is None:
            # No new commits; synthesis-only path already ran
            self._rebuild_db(data_dir)
            active_count = len(json_store.read_all_memories(data_dir))
            return {
                "status": "no_new_commits",
                "commits_processed": 0,
                "total_active_memories": active_count,
            }

        total = len(commits)

        # Phase 3: Batch & confirm cost
        batches, rate_limiter = self._plan_and_confirm_cost(
            commits, token_budget, max_output, auto_confirm,
        )
        if batches is None:
            return {"status": "cancelled", "commits_processed": 0}

        # Phase 4: Extraction (fast model, concurrent)
        new_memories, errors = self._extract_memories(
            batches, data_dir, token_budget, max_output, rate_limiter,
        )

        new_count = len(new_memories)
        if new_count == 0:
            return {
                "status": "success" if not errors else "partial",
                "commits_processed": total,
                "new_memories": 0,
                "updated_memories": 0,
                "new_links": 0,
                "total_active_memories": len(json_store.read_all_memories(data_dir)),
            }

        # Phase 5: Synthesis (reasoning model)
        updated_count, link_count = self._run_synthesis(
            new_memories, data_dir, errors,
        )
        new_count -= (len(new_memories) - new_count) if new_count < 0 else 0

        # Rebuild DB from JSON files
        self._rebuild_db(data_dir)
        active_count = len(json_store.read_all_memories(data_dir))

        # Record build metadata
        self._build_meta.record(BuildMetaEntry(
            build_type=build_type,
            commit_count=total,
            memory_count=active_count,
        ))

        result_dict: dict = {
            "status": "success" if not errors else "partial",
            "commits_processed": total,
            "new_memories": new_count,
            "updated_memories": updated_count,
            "new_links": link_count,
            "total_active_memories": active_count,
        }
        if errors:
            result_dict["errors"] = errors
        return result_dict

    def _validate_and_log_models(self) -> tuple[int, int]:
        """Validate the extract model and compute dynamic budget.

        Returns (token_budget, max_output).
        """
        self._llm.validate_model()
        info = self._llm.get_model_info()
        fallback_info = None
        if self._extract_fallback:
            try:
                fallback_info = self._extract_fallback.get_model_info()
            except Exception:
                pass

        token_budget, max_output, _truncation_limit = self._batch_planner.compute_budget(
            info, fallback_info,
        )

        print(
            f"  extract model: {info['name']} "
            f"(context: {info['context_length']:,}, "
            f"max_output: {info['max_completion_tokens']:,})",
            file=sys.stderr, flush=True,
        )
        if fallback_info:
            print(
                f"  extract fallback: {fallback_info['name']}",
                file=sys.stderr, flush=True,
            )
        if self._reasoning_llm:
            r_info = self._reasoning_llm.get_model_info()
            print(
                f"  reasoning model: {r_info['name']}",
                file=sys.stderr, flush=True,
            )
        print(
            f"  budget: {token_budget:,} input / {max_output:,} output",
            file=sys.stderr, flush=True,
        )
        return token_budget, max_output

    def _discover_commits(
        self,
        data_dir: Path,
        limit: int | None,
        synthesis: bool,
    ) -> tuple[list | None, list[str]]:
        """Find unprocessed commits. Returns (commits, all_hashes).

        Self-healing: for each hash in processed.json, checks that at
        least one memory JSON on disk references that commit.  Stale
        hashes (whose memories were lost) are removed so the commit
        gets re-extracted.

        Returns (None, []) if no new commits (synthesis-only path handled here).
        """
        processed = json_store.read_processed(data_dir)
        all_hashes = self._git.get_all_hashes(limit=limit)

        # Self-heal: remove orphan memory files whose source commits
        # no longer exist in the repo (e.g. after rebase/force-push).
        if processed:
            on_disk = json_store.read_all_memories(data_dir)
            all_hash_set = set(all_hashes)
            orphans = [
                mem for mem in on_disk
                if not (set(mem.source_commits or []) & all_hash_set)
            ]
            if orphans:
                deactivated = 0
                for mem in orphans:
                    mem.active = False
                    json_store.update_memory(mem, data_dir)
                    deactivated += 1
                print(
                    f"  self-heal: deactivated {deactivated} orphan memories "
                    f"(source commits no longer in repo)",
                    file=sys.stderr, flush=True,
                )

        unprocessed_hashes = [h for h in all_hashes if h not in processed]

        # ── Full re-synthesis (explicit flag or self-heal) ──
        all_on_disk = json_store.read_all_memories(data_dir)
        needs_synthesis = synthesis

        # Self-heal: detect if synthesis never completed (0 links with memories)
        if not needs_synthesis and not unprocessed_hashes and len(all_on_disk) > 1:
            last_build = self._build_meta.get_last()
            if last_build and last_build.memory_count > 0:
                row = self._db.query_one("SELECT COUNT(*) as cnt FROM memory_links")
                if row and row["cnt"] == 0:
                    print(
                        f"  self-heal: {len(all_on_disk)} memories but 0 links"
                        f" — synthesis incomplete, re-running",
                        file=sys.stderr, flush=True,
                    )
                    needs_synthesis = True

        if needs_synthesis and all_on_disk:
            synth_llm = self._reasoning_llm or self._llm
            print(
                f"  pass 2: re-synthesizing {len(all_on_disk)} memories...",
                file=sys.stderr, flush=True,
            )
            # Skip triage — all memories are established.
            # Run linking directly on every memory against the full corpus.
            # Full serialization gives the LLM richer context for finding relationships.
            corpus_data = self._synthesis._serialize_full(all_on_disk)
            batch_size = self._synthesis._compute_linking_batch_size(
                synth_llm, all_on_disk, corpus_data,
            )
            batches = self._synthesis._make_batches(all_on_disk, batch_size)
            result = SynthesisResult(accepted_ids=[m.id for m in all_on_disk])

            print(
                f"    linking {len(all_on_disk)} memories in {len(batches)} batch(es)...",
                file=sys.stderr, flush=True,
            )

            for batch_idx, batch in enumerate(batches):
                if len(batches) > 1:
                    print(
                        f"    linking batch [{batch_idx + 1}/{len(batches)}] "
                        f"({len(batch)} memories)...",
                        file=sys.stderr, flush=True,
                    )
                link_result = self._synthesis._run_linking_batch(
                    synth_llm, batch, corpus_data,
                )
                if link_result and "error" not in link_result:
                    result.update_existing.extend(link_result.get("update_existing", []))
                    result.new_links.extend(link_result.get("new_links", []))

            self._synthesis.apply_results(
                result, all_on_disk, data_dir,
                links_only=True,
            )

        # ── No new commits to extract ──
        if not unprocessed_hashes:
            return None, []

        commits = self._git.get_commits_by_hashes(unprocessed_hashes)

        # Filter ignored paths
        path_filter = PathFilter.from_settings(self._config)
        if path_filter.patterns:
            original_count = len(commits)
            filtered_commits = []
            for c in commits:
                fc = path_filter.filter_commit(c)
                if fc is not None:
                    filtered_commits.append(fc)
            if len(filtered_commits) < original_count:
                print(
                    f"  path filter: {original_count - len(filtered_commits)} "
                    f"commits fully ignored, {len(filtered_commits)} remaining",
                    file=sys.stderr, flush=True,
                )
            commits = filtered_commits

        # If path filter removed everything, mark as processed and return
        if not commits:
            json_store.add_processed(set(unprocessed_hashes), data_dir)
            return None, []

        return commits, all_hashes

    def _plan_and_confirm_cost(
        self,
        commits: list,
        token_budget: int,
        max_output: int,
        auto_confirm: bool,
    ) -> tuple[list | None, object]:
        """Batch commits, estimate cost, and prompt for confirmation.

        Returns (batches, rate_limiter) or (None, None) if cancelled.
        """
        max_commits = self._config.batching_max_commits()
        batches = self._batch_planner.make_batches(
            commits, token_budget, max_commits,
        )

        total_input_tokens = sum(
            sum(BatchPlanner.estimate_commit_tokens(c) for c in batch)
            for batch in batches
        )
        est_cost = self._openrouter.estimate_cost(
            self._llm.model, total_input_tokens,
        )
        rate_limiter = self._openrouter.create_rate_limiter(self._llm.model)
        info = self._llm.get_model_info()

        print(
            f"  rate limit: {rate_limiter.rpm} RPM "
            f"({'free model' if info.get('is_free') else 'paid model'})",
            file=sys.stderr, flush=True,
        )
        print(
            f"  pass 1: extracting memories from {len(batches)} batches "
            f"(paced: {rate_limiter.rpm} RPM)",
            file=sys.stderr, flush=True,
        )

        if est_cost > 0:
            print(
                f"  estimated extraction cost: ~${est_cost:.3f} "
                f"({total_input_tokens:,} input tokens)",
                file=sys.stderr, flush=True,
            )
            if not auto_confirm:
                try:
                    answer = input("  proceed? [Y/n] ").strip().lower()
                    if answer and answer not in ("y", "yes"):
                        return None, None
                except (EOFError, KeyboardInterrupt):
                    return None, None
        elif info.get("is_free"):
            print(
                "  estimated extraction cost: FREE",
                file=sys.stderr, flush=True,
            )

        return batches, rate_limiter

    def _extract_memories(
        self,
        batches: list,
        data_dir: Path,
        token_budget: int,
        max_output: int,
        rate_limiter,
    ) -> tuple[list[Memory], list[str]]:
        """Run extraction across batches in a thread pool.

        Returns (new_memories, errors).
        """
        print_lock = threading.Lock()
        errors: list[str] = []
        completed_count = 0
        total_memories_extracted = 0
        build_start = time.time()

        batch_hashes: dict[int, set[str]] = {}
        for i, batch in enumerate(batches, 1):
            batch_hashes[i] = {c.hash for c in batch if c.hash}

        def _llm_extract(batch_num_batch):
            """Run LLM call in thread — returns (batch_num, result dict)."""
            nonlocal completed_count, total_memories_extracted
            batch_num, batch = batch_num_batch

            commits_text = BatchPlanner.format_commits(batch)
            user_msg = f"```json\n{commits_text}\n```"

            # Route oversized batches to fallback model
            batch_tokens = sum(BatchPlanner.estimate_commit_tokens(c) for c in batch)
            if batch_tokens > token_budget and self._extract_fallback:
                llm = self._extract_fallback
                fallback: LLMClient | None = None
                label = f"batch {batch_num}/{len(batches)} [fallback]"
            else:
                llm = self._llm
                fallback = self._extract_fallback
                label = f"batch {batch_num}/{len(batches)}"

            result = call_with_retries(
                llm,
                [
                    {"role": "system", "content": InternalSettings.extraction_system_prompt()},
                    {"role": "user", "content": user_msg},
                ],
                max_tokens=max_output,
                response_schema=InternalSettings.extraction_schema(),
                fallback_llm=fallback,
                label=label,
                print_lock=print_lock,
                rate_limiter=rate_limiter,
            )

            batch_memories: list[Memory] = []
            usage = getattr(llm, 'last_usage', {})

            with print_lock:
                if result and "error" not in result:
                    try:
                        validate_extraction(result)
                    except LLMOutputError as ve:
                        errors.append(f"batch {batch_num}: validation failed: {ve}")
                        return batch_num, batch_memories

                    valid_hashes = batch_hashes.get(batch_num, set())
                    for mem_data in result.get("new_memories", []):
                        mem = MemoryFactory.from_llm_output(mem_data)

                        # Fix hallucinated hashes — only keep hashes that
                        # actually exist in this batch's commits
                        if valid_hashes:
                            real = [h for h in mem.source_commits if h in valid_hashes]
                            if not real:
                                # LLM hallucinated all hashes — use batch hashes
                                mem.source_commits = sorted(valid_hashes)
                            elif len(real) < len(mem.source_commits):
                                mem.source_commits = real

                        json_store.write_memory(mem, data_dir)
                        batch_memories.append(mem)
                elif result and "error" in result:
                    errors.append(result["error"])

                hashes = batch_hashes.get(batch_num, set())
                if hashes:
                    json_store.add_processed(hashes, data_dir)

                completed_count += 1
                total_memories_extracted += len(batch_memories)
                self._log_batch_progress(
                    completed_count, total_memories_extracted,
                    len(batch_memories), len(batch), len(batches),
                    build_start, usage,
                )

            return batch_num, batch_memories

        new_memories: list[Memory] = []
        if not batches:
            return new_memories, errors
        max_workers = min(len(batches), rate_limiter.rpm)
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = [
                executor.submit(_llm_extract, (i, batch))
                for i, batch in enumerate(batches, 1)
            ]
            for future in as_completed(futures):
                _batch_num, batch_mems = future.result()
                new_memories.extend(batch_mems)

        return new_memories, errors

    def _run_synthesis(
        self,
        new_memories: list[Memory],
        data_dir: Path,
        errors: list[str],
    ) -> tuple[int, int]:
        """Run synthesis pass on new memories.

        Returns (updated_count, link_count).
        """
        updated_count = 0
        link_count = 0

        all_on_disk = json_store.read_all_memories(data_dir)
        new_ids = {m.id for m in new_memories}
        existing_memories = [m for m in all_on_disk if m.id not in new_ids]
        synth_llm = self._reasoning_llm or self._llm

        print(
            f"  pass 2: synthesizing {len(new_memories)} new memories "
            f"against {len(existing_memories)} existing...",
            file=sys.stderr, flush=True,
        )

        synth_result = self._synthesis.run(
            synth_llm, new_memories, existing_memories,
        )

        if synth_result.error is None:
            counts = self._synthesis.apply_results(
                synth_result, new_memories, data_dir,
            )
            updated_count = counts["updated"]
            link_count = counts["links"]
        else:
            # Partial results may still have usable data
            if synth_result.accepted_ids or synth_result.new_links:
                counts = self._synthesis.apply_results(
                    synth_result, new_memories, data_dir,
                )
                updated_count = counts["updated"]
                link_count = counts["links"]
            errors.append(synth_result.error)

        return updated_count, link_count

    def _rebuild_db(self, data_dir: Path) -> None:
        """Rebuild the SQLite DB from JSON memory files."""
        from src.memory.db_rebuild import rebuild_db_from_json

        rebuild_db_from_json(
            self._db,
            self._memories,
            self._links,
            data_dir,
            config=self._config,
        )

