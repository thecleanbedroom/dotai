"""Batch planning — token estimation, commit batching, oversized commit splitting.

Extracted from BuildAgent to satisfy Single Responsibility:
BuildAgent orchestrates; BatchPlanner handles sizing decisions.
"""

import json
import sys
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from src.config.settings import Settings

from src.config.internal import InternalSettings
from src.utils import estimate_tokens, split_diff_by_file
from src.memory.models import ParsedCommit


class BatchPlanner:
    """Plans how to divide commits into LLM-sized batches.

    Responsibilities:
      - Estimate token counts for commits
      - Split commit lists into budget-respecting batches
      - Split oversized individual commits by file boundaries
      - Format commits for LLM prompts
    """

    def __init__(self, config: "Settings"):
        self._config = config

    @staticmethod
    def estimate_commit_tokens(commit: ParsedCommit) -> int:
        """Rough token estimate for a single commit."""
        text = (
            commit.hash + commit.author + commit.date
            + commit.message + commit.body + commit.diff
            + "".join(commit.files)
            + "".join(k + v for k, v in commit.trailers.items())
        )
        return estimate_tokens(text)

    def compute_budget(self, llm_info: dict,
                       fallback_info: dict | None = None,
                       ) -> tuple[int, int, int]:
        """Compute (batch_budget, max_output_tokens, truncation_limit).

        batch_budget: max input tokens per batch for the primary model.
        max_output: output tokens to reserve.
        truncation_limit: max input tokens before truncation is needed,
            using the larger of primary/fallback model contexts.
        """
        ctx = llm_info["context_length"]
        model_max_output = llm_info["max_completion_tokens"]

        # Output: use model's max but cap at 1/3 of context to leave room
        max_output = min(model_max_output, ctx // 3)
        max_output = max(max_output, InternalSettings.extraction_min_output_tokens())

        # Input budget for primary model
        max_input = ctx - max_output - InternalSettings.extraction_overhead_tokens()
        budget = min(self._config.batching_token_budget(), max_input)

        # Truncation limit: use the largest model context available
        max_ctx = ctx
        if fallback_info:
            fb_ctx = fallback_info.get("context_length", 0)
            max_ctx = max(max_ctx, fb_ctx)
        truncation_limit = max_ctx - max_output - InternalSettings.extraction_overhead_tokens()

        return budget, max_output, truncation_limit

    def make_batches(self, commits: list[ParsedCommit],
                     budget: int,
                     max_commits: int = 10,
                     truncation_limit: int = 0) -> list[list[ParsedCommit]]:
        """Split commits into batches.

        Splits when either the token budget OR max commits per batch is hit.
        Oversized commits are split into multiple sub-commits by files+body.
        """
        batches: list[list[ParsedCommit]] = []
        current_batch: list[ParsedCommit] = []
        current_tokens = 0

        for commit in commits:
            tokens = self.estimate_commit_tokens(commit)

            # If a single commit exceeds budget, split it into sub-commits
            if tokens > budget:
                if current_batch:
                    batches.append(current_batch)
                    current_batch = []
                    current_tokens = 0

                sub_commits = self.split_oversized_commit(
                    commit, budget, truncation_limit=truncation_limit,
                )
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
    def split_oversized_commit(commit: ParsedCommit,
                               budget: int,
                               truncation_limit: int = 0) -> list[ParsedCommit]:
        """Split a commit that exceeds the token budget into smaller sub-commits.

        Strategy: split by files. Each sub-commit gets the same commit
        metadata (hash, author, date, message) but a subset of files and
        a proportional slice of the body text.
        """
        meta_chars = (
            len(commit.hash) + len(commit.author) + len(commit.date)
            + len(commit.message)
            + sum(len(k) + len(v) for k, v in commit.trailers.items())
            + 120
        )
        meta_tokens = meta_chars // InternalSettings.token_chars_per_token()

        available = max(budget - meta_tokens, budget // 2)
        available_chars = available * InternalSettings.token_chars_per_token()

        diff_by_file = split_diff_by_file(commit.diff)

        sub_commits: list[ParsedCommit] = []
        current_files: list[str] = []
        current_diffs: list[str] = []
        current_chars = 0

        for f in commit.files:
            file_diff = diff_by_file.get(f, "")
            file_chars = len(f) + len(file_diff) + 2

            # Only truncate if diff exceeds the max model context (last resort)
            trunc_chars = (truncation_limit or budget) * 4
            if file_chars > trunc_chars:
                max_diff_chars = trunc_chars - len(f) - 100
                if max_diff_chars > 0 and file_diff:
                    file_diff = file_diff[:max_diff_chars] + "\n... [diff truncated]"
                    file_chars = len(f) + len(file_diff) + 2

            if current_files and current_chars + file_chars > available_chars:
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

        # Handle body text
        body = commit.body
        if body:
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
            sub_commits.append(ParsedCommit(
                hash=commit.hash,
                author=commit.author,
                date=commit.date,
                message=f"{commit.message} [part {len(sub_commits) + 1}]",
                body="",
                files=current_files,
                trailers=commit.trailers,
            ))

        if not sub_commits:
            sub_commits = [commit]

        print(
            f"    split oversized commit {commit.hash[:8]} into "
            f"{len(sub_commits)} sub-batches",
            file=sys.stderr, flush=True,
        )
        return sub_commits

    @staticmethod
    def format_commits(commits: list[ParsedCommit]) -> str:
        """Format commits as JSON for the LLM prompt."""
        entries = []
        for c in commits:
            entry: dict = {
                "hash": c.hash,
                "date": c.date,
                "message": c.message,
            }
            if c.body:
                entry["body"] = c.body
            if c.trailers:
                entry["trailers"] = c.trailers
            if c.files:
                entry["files"] = c.files
            if c.diff:
                entry["diff"] = c.diff
            entries.append(entry)
        return json.dumps(entries)
