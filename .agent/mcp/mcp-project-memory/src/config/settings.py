"""User-configurable settings — every value can be overridden by env var.

For internal non-configurable constants, see internal.py.
"""

import os


class Settings:
    """User-facing settings for the memory system.

    Every value is accessed via a method. Each method checks for a
    constructor override, then an environment variable, then returns
    its default.

    Settings are grouped by entity:
        api_*         — API credentials and endpoint
        extraction_*  — Extraction pass model selection
        synthesis_*   — Synthesis pass model selection
        batching_*    — Commit batching controls
        model_*       — Model capability constraints
        filter_*      — Path filtering rules
    """

    def __init__(self, **overrides: str | int) -> None:
        self._overrides = overrides

    def _env(self, env_var: str, default: str | int) -> str | int:
        """Check override, then env var, then return default."""
        if env_var.lower() in self._overrides:
            return self._overrides[env_var.lower()]
        env_val = os.environ.get(env_var)
        if env_val is not None:
            return int(env_val) if isinstance(default, int) else env_val
        return default

    # ── API ──

    def api_key(self) -> str:
        """OpenRouter API key for LLM requests.

        Used by OpenRouterAPI to authenticate. Required for builds — checked
        early by DependencyChecker. Env: OPENROUTER_API_KEY
        """
        return str(self._env("OPENROUTER_API_KEY", ""))

    def api_url(self) -> str:
        """Base URL for the LLM chat completions endpoint.

        Used by OpenRouterAPI as the request target. Env: MEMORY_BUILD_API_URL
        """
        return str(self._env("MEMORY_BUILD_API_URL", "https://openrouter.ai/api/v1/chat/completions"))

    # ── Extraction (Pass 1) ──

    def extraction_model(self) -> str:
        """Primary LLM model for memory extraction.

        Default model passed to LLMClient. Used for all extraction calls
        unless a fallback is needed. Env: MEMORY_EXTRACT_MODEL
        """
        return str(self._env("MEMORY_EXTRACT_MODEL", "nvidia/nemotron-3-super-120b-a12b:free"))

    def extraction_fallback_model(self) -> str:
        """Fallback LLM model when the primary extraction model fails.

        Used by BuildAgent to create a fallback LLMClient for retries.
        Env: MEMORY_EXTRACT_FALLBACK_MODEL
        """
        return str(self._env("MEMORY_EXTRACT_FALLBACK_MODEL", "google/gemini-2.5-flash-lite"))

    # ── Synthesis (Pass 2) ──

    def synthesis_model(self) -> str:
        """LLM model for the synthesis/dedup pass.

        Used by BuildAgent when creating the synthesis LLMClient.
        Env: MEMORY_REASONING_MODEL
        """
        return str(self._env("MEMORY_REASONING_MODEL", "google/gemini-3.1-pro-preview"))

    # ── Batching ──

    def batching_commit_limit(self) -> int:
        """Max commits to process per build (0 = all unprocessed).

        Used by BuildAgent.run() and run_incremental() to cap how many
        commits are extracted in a single run. Env: MEMORY_COMMIT_LIMIT
        """
        return int(self._env("MEMORY_COMMIT_LIMIT", 0))

    def batching_token_budget(self) -> int:
        """Max input tokens per extraction batch.

        Used by BatchPlanner.compute_budget() to cap the input size sent
        to the LLM per batch. Env: MEMORY_BATCH_TOKEN_BUDGET
        """
        return int(self._env("MEMORY_BATCH_TOKEN_BUDGET", 100000))

    def batching_max_commits(self) -> int:
        """Max commits per extraction batch.

        Used by BuildAgent._extract_batch() to limit how many commits
        are grouped into a single LLM call. Env: MEMORY_BATCH_MAX_COMMITS
        """
        return int(self._env("MEMORY_BATCH_MAX_COMMITS", 20))

    # ── Model constraints ──

    def model_min_context_length(self) -> int:
        """Minimum acceptable context length for LLM models.

        Checked by OpenRouterAPI when querying model info — models below
        this threshold are rejected. Env: MIN_CONTEXT_LENGTH
        """
        return int(self._env("MIN_CONTEXT_LENGTH", 32000))

    # ── Path filtering ──

    def filter_ignore_paths(self) -> str:
        """Comma-separated glob patterns for paths to exclude from extraction.

        Used by PathFilter.from_settings() to build exclusion rules.
        Env: MEMORY_IGNORE_PATHS
        """
        return str(self._env("MEMORY_IGNORE_PATHS", ".agent/memory/data/*"))

    @classmethod
    def load(cls, **overrides: str | int) -> "Settings":
        """Create a Settings instance with optional overrides.

        Convenience factory — equivalent to Settings(**overrides).
        """
        return cls(**overrides)
