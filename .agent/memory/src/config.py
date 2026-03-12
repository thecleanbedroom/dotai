"""Centralized configuration with defaults — single source of truth for all settings."""

import os
from dataclasses import dataclass, fields


@dataclass
class Config:
    """All project-memory settings with sensible defaults.

    Field names match env var names exactly (uppercase) so it's clear
    where they come from. Resolution: explicit arg > env var > default.
    """

    # API
    OPENROUTER_API_KEY: str = ""
    MEMORY_BUILD_API_URL: str = "https://openrouter.ai/api/v1/chat/completions"
    MEMORY_EXTRACT_MODEL: str = "nvidia/nemotron-3-super-120b-a12b:free"
    MEMORY_EXTRACT_FALLBACK_MODEL: str = "google/gemini-2.5-flash-lite"
    MEMORY_REASONING_MODEL: str = "google/gemini-3.1-pro-preview"

    # Build batching
    MEMORY_COMMIT_LIMIT: int = 0          # 0 = all commits
    MEMORY_BATCH_TOKEN_BUDGET: int = 100000
    MEMORY_BATCH_MAX_COMMITS: int = 20

    # Model constraints
    MIN_CONTEXT_LENGTH: int = 32000

    @classmethod
    def from_env(cls, **overrides) -> "Config":
        """Create a Config, filling unset values from environment variables.

        Explicit overrides take priority, then env vars, then class defaults.
        """
        kwargs = {}
        defaults = cls()
        for f in fields(cls):
            if f.name in overrides:
                kwargs[f.name] = overrides[f.name]
            else:
                env_val = os.environ.get(f.name)
                if env_val is not None:
                    default_val = getattr(defaults, f.name)
                    if isinstance(default_val, int):
                        kwargs[f.name] = int(env_val)
                    else:
                        kwargs[f.name] = env_val

        return cls(**kwargs)
