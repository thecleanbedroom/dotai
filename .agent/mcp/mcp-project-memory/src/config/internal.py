"""Internal constants — prompts, schemas, and non-configurable settings.

These are implementation details not exposed to users.
For user-configurable settings, see settings.py.
"""

import json as _json
from pathlib import Path as _Path


class InternalSettings:
    """All internal operational switches for the memory system.

    Single source of truth for every non-user-facing constant.
    Every value is accessed via a classmethod — no bare attribute access.

    Settings are grouped by entity:
        token_*        — Token estimation used by estimate_tokens() in utils.py
        extraction_*   — Extraction pass (batching budgets, prompt, schema)
        synthesis_*    — Synthesis pass (batch sizing, prompt, schema)
        retry_*        — LLM call retry/resilience behavior
        confidence_*   — Confidence scoring thresholds in MemoryFactory
        validation_*   — LLM output validation rules in validators.py
    """

    # ── Private infrastructure ──

    _cache_text: dict[str, str] = {}
    _cache_json: dict[str, dict] = {}

    @classmethod
    def _config_dir(cls) -> _Path:
        return _Path(__file__).parent

    @classmethod
    def _load_text(cls, key: str, path: _Path) -> str:
        """Load and cache a text file."""
        if key not in cls._cache_text:
            cls._cache_text[key] = path.read_text().strip()
        return cls._cache_text[key]

    @classmethod
    def _json_rules(cls) -> str:
        """Shared JSON output safety rules — appended to all LLM prompts."""
        return cls._load_text("json_rules", cls._config_dir() / "prompts" / "prompt_json_rules.md")

    @classmethod
    def _load_json(cls, key: str, path: _Path) -> dict:
        """Load and cache a JSON file."""
        if key not in cls._cache_json:
            cls._cache_json[key] = _json.loads(path.read_text())
        return cls._cache_json[key]

    # ── Token estimation ──

    @classmethod
    def token_chars_per_token(cls) -> int:
        """Characters per token ratio for estimate_tokens() in utils.py.

        Used by BatchPlanner to convert between character counts and token
        budgets when sizing diffs and computing available capacity.
        """
        return 4

    # ── Extraction (Pass 1) ──

    @classmethod
    def extraction_overhead_tokens(cls) -> int:
        """Tokens reserved for system prompt, message framing, and other overhead.

        Subtracted from model context when computing the input budget and
        truncation limit in BatchPlanner.compute_budget().
        """
        return 8_000

    @classmethod
    def extraction_min_output_tokens(cls) -> int:
        """Minimum output tokens reserved for extraction LLM responses.

        Floor for max_output in BatchPlanner.compute_budget() — ensures the
        model always has enough room to produce a full extraction even when
        the context window is mostly consumed by input.
        """
        return 4_000

    @classmethod
    def extraction_system_prompt(cls) -> str:
        """System prompt sent to the LLM during extraction.

        Loaded from prompts/prompt_extract_system.md. Instructs the LLM on
        memory types, scoring rules, tag conventions, and output format.
        """
        return cls._load_text("extraction_prompt", cls._config_dir() / "prompts" / "prompt_extract_system.md") + "\n\n" + cls._json_rules()

    @classmethod
    def extraction_schema(cls) -> dict:
        """JSON schema for structured extraction output.

        Loaded from schemas/schema_extract.json. Passed to the LLM as
        response_schema to enforce structured output format.
        """
        return cls._load_json("extraction_schema", cls._config_dir() / "schemas" / "schema_extract.json")

    # ── Synthesis (Pass 2) ──

    @classmethod
    def synthesis_context_fill_ratio(cls) -> float:
        """Fraction of model context to fill when sizing linking batches.

        Used by _compute_linking_batch_size() to determine how much of the
        remaining context (after corpus + system prompt + output reserve)
        to fill with batch memories. Higher = fewer, larger batches.
        """
        return 0.5

    @classmethod
    def synthesis_triage_prompt(cls) -> str:
        """System prompt for Phase 1 — triage (accept/reject/deactivate).

        Loaded from prompts/prompt_synthesis_triage.md.
        """
        return cls._load_text("triage_prompt", cls._config_dir() / "prompts" / "prompt_synthesis_triage.md") + "\n\n" + cls._json_rules()

    @classmethod
    def synthesis_triage_schema(cls) -> dict:
        """JSON schema for Phase 1 — triage output.

        Loaded from schemas/schema_synthesis_triage.json.
        """
        return cls._load_json("triage_schema", cls._config_dir() / "schemas" / "schema_synthesis_triage.json")

    @classmethod
    def synthesis_linking_prompt(cls) -> str:
        """System prompt for Phase 2 — linking and updates.

        Loaded from prompts/prompt_synthesis_linking.md.
        """
        return cls._load_text("linking_prompt", cls._config_dir() / "prompts" / "prompt_synthesis_linking.md") + "\n\n" + cls._json_rules()

    @classmethod
    def synthesis_linking_schema(cls) -> dict:
        """JSON schema for Phase 2 — linking output.

        Loaded from schemas/schema_synthesis_linking.json.
        """
        return cls._load_json("linking_schema", cls._config_dir() / "schemas" / "schema_synthesis_linking.json")

    # ── Retry / resilience ──

    @classmethod
    def retry_max_retries(cls) -> int:
        """Maximum number of LLM call attempts before giving up.

        Default for max_retries parameter in call_with_retries().
        """
        return 4

    @classmethod
    def retry_rate_limit_base_wait(cls) -> int:
        """Base wait seconds for rate-limit retries (exponential backoff).

        Multiplied by 2^attempt in call_with_retries() when a rate-limit
        error (HTTP 429) is encountered.
        """
        return 15

    @classmethod
    def retry_transient_base_wait(cls) -> int:
        """Base wait seconds for transient error retries (linear backoff).

        Added to attempt number in call_with_retries() for non-rate-limit
        transient errors (timeouts, 5xx).
        """
        return 1

    # ── Confidence scoring thresholds (MemoryFactory) ──

    @classmethod
    def confidence_commits_thresholds(cls) -> dict:
        """Source commit count → confidence points mapping.

        Used by MemoryFactory to award confidence points based on how many
        commits support a memory. Format: {min_count: points_awarded}.
        """
        return {1: 8, 2: 20, 3: 30}

    @classmethod
    def confidence_files_thresholds(cls) -> dict:
        """File count → confidence points mapping.

        Used by MemoryFactory to award confidence points based on how many
        files a memory touches. Format: {min_count: points_awarded}.
        """
        return {1: 5, 2: 15, 4: 25, 7: 30}

    @classmethod
    def confidence_summary_thresholds(cls) -> dict:
        """Summary character length → confidence points mapping.

        Used by MemoryFactory to award confidence points based on summary
        detail/length. Format: {min_chars: points_awarded}.
        """
        return {100: 5, 200: 12, 300: 20}

    @classmethod
    def confidence_tags_thresholds(cls) -> dict:
        """Tag count → confidence points mapping.

        Used by MemoryFactory to award confidence points based on how many
        tags a memory has. Format: {min_count: points_awarded}.
        """
        return {3: 5, 5: 12, 7: 20}

    # ── Validation ──

    @classmethod
    def validation_min_commit_hash_length(cls) -> int:
        """Minimum character length for commit hashes in LLM output.

        Checked by _validate_memory_dict() in validators.py to reject
        obviously truncated or garbage commit hashes.
        """
        return 4

    @classmethod
    def validation_max_importance(cls) -> int:
        """Upper bound for memory importance scores (0 to this value).

        Checked by _validate_memory_dict() in validators.py to reject
        out-of-range importance values from LLM output.
        """
        return 100
