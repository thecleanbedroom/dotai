"""Build agent — orchestrates commit processing, LLM calls, and memory creation."""

import json
import os
import sys
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from src.config import Config

from src.models import Memory, MemoryLink, BuildMetaEntry, ParsedCommit
from src.db import Database
from src.stores import MemoryStore, LinkStore, BuildMetaStore
from src.git import GitLogParser
from src.llm import LLMClient
from src.openrouter import OpenRouterAPI, RateLimiter

from src.prompts import (
    EXTRACT_SYSTEM_PROMPT as BUILD_SYSTEM_PROMPT,
    EXTRACT_SYSTEM_PROMPT,
    SYNTHESIS_SYSTEM_PROMPT,
    EXTRACT_SCHEMA,
    SYNTHESIS_SCHEMA,
)


def _is_http_transient(e: Exception) -> bool:
    """Check if an exception is a transient HTTP error (429 or 5xx)."""
    try:
        from requests.exceptions import HTTPError
        if isinstance(e, HTTPError) and e.response is not None:
            return e.response.status_code == 429 or e.response.status_code >= 500
    except ImportError:
        pass
    return False


class BuildAgent:
    """Orchestrates build: parse commits → LLM call → memory creation → DB writes.

    Two-pass architecture:
      Pass 1 (extraction): Fast model extracts memories from commit batches.
      Pass 2 (synthesis): Reasoning model links, deduplicates, and adjusts memories.
    """

    def __init__(
        self,
        db: Database,
        memory_store: MemoryStore,
        link_store: LinkStore,
        build_meta_store: BuildMetaStore,
        git_parser: GitLogParser,
        llm_client: LLMClient,
        config: Optional["Config"] = None,
        *,
        extract_fallback_llm: Optional[LLMClient] = None,
        reasoning_llm: Optional[LLMClient] = None,
        openrouter: Optional[OpenRouterAPI] = None,
    ):
        if config is None:
            from src.config import Config
            config = Config.from_env()
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

    def build(self, *, limit: Optional[int] = None, auto_confirm: bool = False) -> dict:
        """Incremental build — process commits since the last build.

        Safe to interrupt (Ctrl+C): progress is recorded after each
        successful build, so the next `build` resumes from where it
        left off. If no previous build exists, processes all history.

        Args:
            limit: Max commits to process (newest first). Reads from
                   MEMORY_COMMIT_LIMIT env var if not provided.
                   0 or None = all new commits.
            auto_confirm: Skip cost confirmation prompt (--yes flag).
        """
        limit = limit or self._config.MEMORY_COMMIT_LIMIT or None

        # Find where the last build left off
        last_build = self._build_meta.get_last()
        since_hash = last_build.last_commit if last_build and last_build.last_commit else None

        return self._run_build(
            since_hash=since_hash,
            limit=limit,
            build_type="incremental" if since_hash else "full",
            auto_confirm=auto_confirm,
        )

    def rebuild(self, *, limit: Optional[int] = None, auto_confirm: bool = False) -> dict:
        """Full rebuild — drop all data and reprocess entire git history.

        Backs up the existing DB first. If the rebuild produces zero
        memories (total failure), restores the backup.

        Args:
            limit: Max commits to process (newest first). Reads from
                   MEMORY_COMMIT_LIMIT env var if not provided.
                   0 or None = all commits.
            auto_confirm: Skip cost confirmation prompt (--yes flag).
        """
        limit = limit or self._config.MEMORY_COMMIT_LIMIT or None
        import shutil

        db_path = self._db.db_path
        is_file_db = db_path != ":memory:" and os.path.exists(db_path)
        backup_path = f"{db_path}.bak" if is_file_db else None

        if is_file_db:
            # Backup existing DB before wiping
            shutil.copy2(db_path, backup_path)  # type: ignore[arg-type]

        # Wipe all tables in-place
        self._db.drop_all()
        self._db.init_schema()

        # Clear old build response logs
        responses_dir = os.path.join(
            os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
            "data", "build_responses",
        )
        if os.path.isdir(responses_dir):
            for f in os.listdir(responses_dir):
                fp = os.path.join(responses_dir, f)
                if os.path.isfile(fp):
                    os.remove(fp)

        result = self._run_build(
            since_hash=None,
            limit=limit,
            build_type="full",
            auto_confirm=auto_confirm,
        )

        # Restore backup if rebuild produced nothing
        if result.get("new_memories", 0) == 0 and backup_path and os.path.exists(backup_path):
            self._db.close()
            if os.path.exists(db_path):
                os.remove(db_path)
            shutil.move(backup_path, db_path)
            self._db.__init__(db_path)  # type: ignore[misc]
            self._db.init_schema()
            result["status"] = "failed_restored"
            result["message"] = "Rebuild failed — no memories created. Previous DB restored."
            print("  rebuild failed, previous DB restored", file=sys.stderr, flush=True)
        elif backup_path and os.path.exists(backup_path):
            os.remove(backup_path)

        return result

    # Overhead tokens for system prompt + existing-memories context
    _OVERHEAD_TOKENS = 8_000
    # Minimum output tokens to reserve for the response
    _MIN_OUTPUT_TOKENS = 4_000

    def _compute_budget(self) -> tuple[int, int]:
        """Compute (batch_budget, max_output_tokens) from model capabilities.

        Uses config batch_token_budget. Output tokens auto-tuned from model.
        Budget is capped so input + output + overhead fits within context.
        """
        info = self._llm.get_model_info()
        ctx = info["context_length"]
        model_max_output = info["max_completion_tokens"]

        # Output: use model's max but cap at 1/3 of context to leave room
        max_output = min(model_max_output, ctx // 3)
        max_output = max(max_output, self._MIN_OUTPUT_TOKENS)

        # Input budget: never exceed what the model can actually handle
        max_input = ctx - max_output - self._OVERHEAD_TOKENS
        budget = min(self._config.MEMORY_BATCH_TOKEN_BUDGET, max_input)

        return budget, max_output

    def _run_build(
        self,
        *,
        since_hash: Optional[str],
        limit: Optional[int],
        build_type: str,
        auto_confirm: bool = False,
    ) -> dict:
        """Core build logic — two-pass architecture.

        Pass 1 (extraction): Fast model processes commit batches → new memories.
        Pass 2 (synthesis): Reasoning model links, deduplicates, adjusts all memories.
        """
        # Validate model and compute dynamic budget
        self._llm.validate_model()
        token_budget, max_output = self._compute_budget()
        info = self._llm.get_model_info()
        print(
            f"  extract model: {info['name']} "
            f"(context: {info['context_length']:,}, "
            f"max_output: {info['max_completion_tokens']:,})",
            file=sys.stderr, flush=True,
        )
        if self._extract_fallback:
            fb_info = self._extract_fallback.get_model_info()
            print(
                f"  extract fallback: {fb_info['name']}",
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

        # Get commits
        try:
            raw_log = self._git.get_file_list(since_hash=since_hash, limit=limit)
        except RuntimeError as e:
            if "Invalid revision range" in str(e) and since_hash:
                print(
                    f"  warning: last_commit {since_hash[:12]} not found in repo, "
                    f"falling back to full scan",
                    file=sys.stderr, flush=True,
                )
                since_hash = None
                raw_log = self._git.get_file_list(since_hash=None, limit=limit)
            else:
                raise
        commits = self._git.parse(raw_log)
        if limit:
            commits.reverse()  # git returned newest-first, we want chronological

        if not commits:
            return {"status": "no_new_commits", "commits_processed": 0}

        # Split into batches by token budget + commit count limit
        max_commits = self._config.MEMORY_BATCH_MAX_COMMITS
        batches = self._make_batches(commits, token_budget, max_commits)
        total = len(commits)
        new_count = 0
        errors: list[str] = []

        # ── Pass 1: Extraction (fast model, concurrent) ──

        # Estimate cost and show summary
        total_input_tokens = sum(
            sum(self._estimate_commit_tokens(c) for c in batch)
            for batch in batches
        )
        info = self._llm.get_model_info()
        est_cost = self._openrouter.estimate_cost(
            self._llm.model, total_input_tokens,
        )
        rate_limiter = self._openrouter.create_rate_limiter(self._llm.model)

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
                        return {"status": "cancelled", "commits_processed": 0}
                except (EOFError, KeyboardInterrupt):
                    return {"status": "cancelled", "commits_processed": 0}
        elif info.get("is_free"):
            print(
                f"  estimated extraction cost: FREE",
                file=sys.stderr, flush=True,
            )

        from concurrent.futures import ThreadPoolExecutor, as_completed
        import threading
        print_lock = threading.Lock()

        def _llm_extract(batch_num_batch):
            """Run LLM call in thread — returns (batch_num, result dict)."""
            batch_num, batch = batch_num_batch

            commits_text = self._format_commits(batch)
            user_msg = f"New commits to process:\n{commits_text}"

            result = self._llm_call_with_retries(
                self._llm,
                [
                    {"role": "system", "content": BUILD_SYSTEM_PROMPT},
                    {"role": "user", "content": user_msg},
                ],
                max_tokens=max_output,
                response_schema=EXTRACT_SCHEMA,
                fallback_llm=self._extract_fallback,
                label=f"batch {batch_num}/{len(batches)} ({len(batch)} commits)",
                print_lock=print_lock,
                rate_limiter=rate_limiter,
            )
            return batch_num, result

        # Workers capped at RPM — more threads just pile up on the pacer lock
        llm_results: list[Optional[dict]] = []
        max_workers = min(len(batches), rate_limiter.rpm)
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = [
                executor.submit(_llm_extract, (i, batch))
                for i, batch in enumerate(batches, 1)
            ]
            for future in as_completed(futures):
                _batch_num, result = future.result()
                llm_results.append(result)

        # Save all extracted memories to DB (main thread, sequential)
        for result in llm_results:
            if result is None:
                continue
            if "error" in result:
                errors.append(result["error"])
                continue
            for mem_data in result.get("new_memories", []):
                self._memories.create(self._memory_from_dict(mem_data))
                new_count += 1

        # ── Pass 2: Synthesis (reasoning model) ──
        updated_count = 0
        deactivated_count = 0
        link_count = 0

        synth_llm = self._reasoning_llm or self._llm
        print(
            f"  pass 2: synthesizing links across {self._memories.count()} memories...",
            file=sys.stderr, flush=True,
        )
        synth_result = self._synthesis_pass(synth_llm)
        if synth_result and "error" not in synth_result:
            updated_count += synth_result.get("updated", 0)
            deactivated_count += synth_result.get("deactivated", 0)
            link_count += synth_result.get("links", 0)
        elif synth_result and "error" in synth_result:
            errors.append(synth_result["error"])

        # Record build — always use git rev-parse HEAD for reliability
        # (the parser can produce phantom commits from trailing git output)
        current_hash = self._git.get_current_hash()
        active_count = self._memories.count()
        self._build_meta.record(BuildMetaEntry(
            build_type=build_type,
            last_commit=current_hash,
            commit_count=total,
            memory_count=active_count,
        ))

        result_dict: dict = {
            "status": "success" if not errors else "partial",
            "commits_processed": total,
            "new_memories": new_count,
            "updated_memories": updated_count,
            "deactivated_memories": deactivated_count,
            "new_links": link_count,
            "total_active_memories": active_count,
        }
        if errors:
            result_dict["errors"] = errors
        return result_dict

    @staticmethod
    def _estimate_commit_tokens(commit: ParsedCommit) -> int:
        """Rough token estimate for a single commit (~4 chars per token)."""
        chars = (
            len(commit.hash) + len(commit.author) + len(commit.date)
            + len(commit.message) + len(commit.body)
            + len(commit.diff)
            + sum(len(f) for f in commit.files)
            + sum(len(k) + len(v) for k, v in commit.trailers.items())
            + 80  # formatting overhead
        )
        return max(chars // 4, 1)

    def _make_batches(self, commits: list[ParsedCommit],
                      budget: int,
                      max_commits: int = 10) -> list[list[ParsedCommit]]:
        """Split commits into batches.

        Splits when either the token budget OR max commits per batch is hit.
        Oversized commits are split into multiple sub-commits by files+body.
        """
        batches: list[list[ParsedCommit]] = []
        current_batch: list[ParsedCommit] = []
        current_tokens = 0

        for commit in commits:
            tokens = self._estimate_commit_tokens(commit)

            # If a single commit exceeds budget, split it into sub-commits
            if tokens > budget:
                # Flush any pending batch first
                if current_batch:
                    batches.append(current_batch)
                    current_batch = []
                    current_tokens = 0

                sub_commits = self._split_oversized_commit(commit, budget)
                for sc in sub_commits:
                    batches.append([sc])
                continue

            if current_batch and (
                current_tokens + tokens > budget
                or len(current_batch) >= max_commits
            ):
                batches.append(current_batch)
                current_batch = []
                current_tokens = 0
            current_batch.append(commit)
            current_tokens += tokens

        if current_batch:
            batches.append(current_batch)
        return batches

    @staticmethod
    def _split_oversized_commit(commit: ParsedCommit,
                                budget: int) -> list[ParsedCommit]:
        """Split a commit that exceeds the token budget into smaller sub-commits.

        Strategy: split by files. Each sub-commit gets the same commit
        metadata (hash, author, date, message) but a subset of files and
        a proportional slice of the body text.
        """
        # Estimate metadata overhead (everything except body and files)
        meta_chars = (
            len(commit.hash) + len(commit.author) + len(commit.date)
            + len(commit.message)
            + sum(len(k) + len(v) for k, v in commit.trailers.items())
            + 120  # formatting overhead
        )
        meta_tokens = meta_chars // 4

        # Available tokens per sub-commit for body + files
        available = max(budget - meta_tokens, budget // 2)
        available_chars = available * 4

        # Split files into groups that fit
        # Split diff into per-file chunks
        diff_by_file: dict[str, str] = {}
        if commit.diff:
            current_file = ""
            current_lines: list[str] = []
            for line in commit.diff.split("\n"):
                if line.startswith("diff --git "):
                    if current_file and current_lines:
                        diff_by_file[current_file] = "\n".join(current_lines)
                    # Extract filename: diff --git a/path b/path
                    parts = line.split(" b/", 1)
                    current_file = parts[1] if len(parts) > 1 else ""
                    current_lines = [line]
                else:
                    current_lines.append(line)
            if current_file and current_lines:
                diff_by_file[current_file] = "\n".join(current_lines)

        sub_commits: list[ParsedCommit] = []
        current_files: list[str] = []
        current_diffs: list[str] = []
        current_chars = 0

        for f in commit.files:
            file_diff = diff_by_file.get(f, "")
            file_chars = len(f) + len(file_diff) + 2

            # Truncate individual file diffs that exceed the budget
            if file_chars > available_chars:
                max_diff_chars = available_chars - len(f) - 100
                if max_diff_chars > 0 and file_diff:
                    file_diff = file_diff[:max_diff_chars] + "\n... [diff truncated]"
                    file_chars = len(f) + len(file_diff) + 2

            if current_files and current_chars + file_chars > available_chars:
                # Create sub-commit with current files
                sub_commits.append(ParsedCommit(
                    hash=commit.hash,
                    author=commit.author,
                    date=commit.date,
                    message=f"{commit.message} [part {len(sub_commits) + 1}]",
                    body="",
                    diff="\n".join(current_diffs),
                    files=current_files,
                    trailers=commit.trailers,
                ))
                current_files = []
                current_diffs = []
                current_chars = 0
            current_files.append(f)
            if file_diff:
                current_diffs.append(file_diff)
            current_chars += file_chars

        # Handle body text — split across sub-commits evenly
        body = commit.body
        if body:
            # If we have remaining files, add them as a sub-commit first
            if current_files:
                sub_commits.append(ParsedCommit(
                    hash=commit.hash,
                    author=commit.author,
                    date=commit.date,
                    message=f"{commit.message} [part {len(sub_commits) + 1}]",
                    body="",
                    diff="\n".join(current_diffs),
                    files=current_files,
                    trailers=commit.trailers,
                ))
                current_files = []
                current_diffs = []

            # Split body into chunks
            body_budget_chars = available_chars
            for i in range(0, len(body), body_budget_chars):
                chunk = body[i:i + body_budget_chars]
                sub_commits.append(ParsedCommit(
                    hash=commit.hash,
                    author=commit.author,
                    date=commit.date,
                    message=f"{commit.message} [body part {len(sub_commits) + 1}]",
                    body=chunk,
                    files=[],
                    trailers=commit.trailers if i == 0 else {},
                ))
        elif current_files:
            # No body, just remaining files
            sub_commits.append(ParsedCommit(
                hash=commit.hash,
                author=commit.author,
                date=commit.date,
                message=f"{commit.message} [part {len(sub_commits) + 1}]",
                body="",
                files=current_files,
                trailers=commit.trailers,
            ))

        # Fallback: if somehow nothing was split, return the original
        if not sub_commits:
            sub_commits = [commit]

        print(
            f"    split oversized commit {commit.hash[:8]} into "
            f"{len(sub_commits)} sub-batches",
            file=sys.stderr, flush=True,
        )
        return sub_commits

    def _llm_call_with_retries(
        self, llm: LLMClient, messages: list[dict],
        *, max_tokens: int, response_schema: dict,
        max_retries: int = 4,
        fallback_llm: Optional[LLMClient] = None,
        label: str = "",
        print_lock: Optional["threading.Lock"] = None,
        rate_limiter: Optional[RateLimiter] = None,
    ) -> Optional[dict]:
        """Make an LLM call with retry logic. Returns parsed dict or error dict.

        On truncation (finish_reason=length), escalates to fallback_llm if provided.
        If rate_limiter is provided, acquires before each attempt and signals on 429.
        """
        last_error = None
        for attempt in range(max_retries):
            try:
                if rate_limiter:
                    rate_limiter.acquire()
                response_text = llm.chat(
                    messages,
                    max_tokens=max_tokens,
                    response_schema=response_schema,
                    label=label,
                    print_lock=print_lock,
                )
                if rate_limiter:
                    rate_limiter.on_success()
                return json.loads(response_text)
            except Exception as e:
                last_error = e

                # Truncation = model's output cap hit.
                # Escalate to a bigger model if available.
                if "finish_reason=length" in str(e):
                    if fallback_llm and fallback_llm is not llm:
                        fb_info = fallback_llm.get_model_info()
                        fb_max = fb_info.get("max_completion_tokens", max_tokens)
                        print(
                            f"      truncated — escalating to {fb_info['name']} "
                            f"(max_output: {fb_max:,})",
                            file=sys.stderr, flush=True,
                        )
                        llm = fallback_llm
                        max_tokens = min(fb_max, fb_info.get("context_length", fb_max) // 3)
                        continue
                    return {"error": (
                        f"Output truncated (model output cap hit). "
                        f"Use a model with a higher max_completion_tokens. {e}"
                    )}

                is_transient = False
                is_rate_limit = False
                if isinstance(e, (json.JSONDecodeError, ValueError)):
                    is_transient = True
                elif _is_http_transient(e):
                    is_transient = True
                    from requests.exceptions import HTTPError
                    if isinstance(e, HTTPError) and e.response is not None:
                        is_rate_limit = e.response.status_code == 429
                elif isinstance(e, (ConnectionError, TimeoutError, OSError)):
                    is_transient = True

                if is_transient and attempt < max_retries - 1:
                    if is_rate_limit:
                        # Let the rate limiter handle 429s globally
                        if rate_limiter:
                            retry_after = None
                            from requests.exceptions import HTTPError
                            if isinstance(e, HTTPError) and e.response is not None:
                                retry_after_str = e.response.headers.get("Retry-After")
                                if retry_after_str:
                                    try:
                                        retry_after = float(retry_after_str)
                                    except ValueError:
                                        pass
                            rate_limiter.on_rate_limit(retry_after)
                            continue  # Rate limiter will block on next acquire()
                        else:
                            wait = 15 * (2 ** attempt)
                    else:
                        wait = 1 + attempt
                    print(
                        f"      retry {attempt + 1}/{max_retries - 1} after {wait}s ({e})",
                        file=sys.stderr, flush=True,
                    )
                    time.sleep(wait)
                    continue
                return {"error": f"call failed: {e}"}
        return {"error": f"call failed after {max_retries} attempts: {last_error}"}

    @staticmethod
    def _memory_from_dict(data: dict) -> Memory:
        """Convert a dict from LLM output into a Memory dataclass.

        Confidence is computed from evidence signals weighted by
        discriminative value. Structural signals (commits, files) are
        weighted higher because they reflect real breadth of change.
        Cosmetic signals (summary length, tags) are weighted lower
        because the LLM inflates them on every extraction.

        Scoring (0-100):
          - source_commits: 0→0, 1→8, 2→20, 3+→30     (max 30, structural)
          - files:          0→0, 1→5, 2-3→15, 4-6→25, 7+→30 (max 30, structural)
          - summary length: ≤100→0, ≤200→5, ≤300→12, 300+→20 (max 20, cosmetic)
          - tags:           ≤2→0, 3-4→5, 5-6→12, 7+→20      (max 20, cosmetic)
        The raw score (0-100) is stored as the confidence value.
        """
        source_commits = data.get("source_commits", [])
        files = data.get("files", [])
        summary = data.get("summary", "")
        tags = data.get("tags", [])

        score = 0

        # Source commits (0-30): rarest signal, most valuable
        n_commits = len(source_commits)
        if n_commits >= 3:
            score += 30
        elif n_commits == 2:
            score += 20
        elif n_commits == 1:
            score += 8

        # Files referenced (0-30): structural, shows breadth of change
        n_files = len(files)
        if n_files >= 7:
            score += 30
        elif n_files >= 4:
            score += 25
        elif n_files >= 2:
            score += 15
        elif n_files == 1:
            score += 5

        # Summary length (0-20): cosmetic, LLM always writes >100 chars
        s_len = len(summary)
        if s_len > 300:
            score += 20
        elif s_len > 200:
            score += 12
        elif s_len > 100:
            score += 5

        # Tags (0-20): cosmetic, LLM always produces 4-5 tags
        n_tags = len(tags)
        if n_tags >= 7:
            score += 20
        elif n_tags >= 5:
            score += 12
        elif n_tags >= 3:
            score += 5

        return Memory(
            summary=summary,
            type=data.get("type", "context"),
            confidence=score,
            importance=data.get("importance", 0.5),
            source_commits=source_commits,
            files=files,
            tags=tags,
        )

    def _synthesis_pass(self, llm: LLMClient) -> Optional[dict]:
        """Pass 2: Synthesize links, updates, and deactivations across all memories.

        Uses the reasoning model. Sees all memories at once for accurate linking.
        """
        all_memories = self._memories.list_all(limit=10_000)
        if not all_memories:
            return {"updated": 0, "deactivated": 0, "links": 0}

        memories_data = [
            {"id": m.id, "summary": m.summary, "type": m.type,
             "confidence": m.confidence, "importance": m.importance,
             "files": m.files, "tags": m.tags}
            for m in all_memories
        ]

        user_msg = (
            f"Analyze these {len(memories_data)} memories and create links, "
            f"updates, and deactivations:\n\n"
            + json.dumps(memories_data, indent=2)
        )

        # Estimate tokens for the response
        input_tokens = len(json.dumps(memories_data)) // 4
        max_output = max(input_tokens // 2, 8_000)  # generous output budget

        result = self._llm_call_with_retries(
            llm,
            [
                {"role": "system", "content": SYNTHESIS_SYSTEM_PROMPT},
                {"role": "user", "content": user_msg},
            ],
            max_tokens=max_output,
            response_schema=SYNTHESIS_SCHEMA,
        )
        if result is None or "error" in result:
            return result

        updated_count = 0
        deactivated_count = 0
        link_count = 0

        # Update existing memories
        for update_data in result.get("update_memories", []):
            mem_id = update_data.get("id")
            if mem_id is None:
                continue
            existing_mem = self._memories.get(mem_id)
            if existing_mem is None:
                continue
            if "summary" in update_data:
                existing_mem.summary = update_data["summary"]
            if "importance" in update_data:
                existing_mem.importance = update_data["importance"]
            self._memories.update(existing_mem)
            updated_count += 1

        # Deactivate memories
        for mem_id in result.get("deactivate_memory_ids", []):
            self._memories.deactivate(mem_id)
            deactivated_count += 1

        # Create links — all IDs are integers now (real DB IDs)
        for link_data in result.get("new_links", []):
            id_a = link_data.get("source")
            id_b = link_data.get("target")

            if not isinstance(id_a, int) or not isinstance(id_b, int):
                print(
                    f"    skip link {id_a}↔{id_b}: not integer IDs",
                    file=sys.stderr, flush=True,
                )
                continue

            # Validate both memories exist
            if self._memories.get(id_a) is None or self._memories.get(id_b) is None:
                print(
                    f"    skip link {id_a}↔{id_b}: memory not found",
                    file=sys.stderr, flush=True,
                )
                continue

            link = MemoryLink(
                memory_id_a=id_a,
                memory_id_b=id_b,
                relationship=link_data.get("relationship", "related_to"),
                strength=link_data.get("strength", 0.5),
            )
            try:
                self._links.create(link)
                link_count += 1
            except Exception as e:
                print(
                    f"    skip link {id_a}↔{id_b}: {e}",
                    file=sys.stderr, flush=True,
                )

        # Auto-deactivate targets of 'supersedes' links
        for link_data in result.get("new_links", []):
            if link_data.get("relationship") == "supersedes":
                target_id = link_data.get("target")
                if isinstance(target_id, int):
                    mem = self._memories.get(target_id)
                    if mem and mem.active:
                        self._memories.deactivate(target_id)
                        deactivated_count += 1

        return {
            "updated": updated_count,
            "deactivated": deactivated_count,
            "links": link_count,
        }

    def _format_commits(self, commits: list[ParsedCommit]) -> str:
        """Format commits for the LLM prompt."""
        parts = []
        for c in commits:
            section = f"=== Commit {c.hash[:8]} ===\n"
            section += f"Author: {c.author}\n"
            section += f"Date: {c.date}\n"
            section += f"Message: {c.message}\n"
            if c.body:
                section += f"Body:\n{c.body}\n"
            if c.trailers:
                section += f"Trailers: {json.dumps(c.trailers)}\n"
            if c.files:
                section += f"Files: {', '.join(c.files)}\n"
            if c.diff:
                section += f"Diff:\n{c.diff}\n"
            parts.append(section)
        return "\n".join(parts)
