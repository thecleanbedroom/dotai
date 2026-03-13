"""OpenRouter API client — model info, pricing, rate limits, and validation."""

from typing import TYPE_CHECKING, Optional

from src.llm.rate_limiter import RateLimiter

if TYPE_CHECKING:
    from src.config.settings import Settings


class OpenRouterAPI:
    """Centralized OpenRouter API access — models, pricing, rate limits.

    All model detection, capability checks, and rate info go through this class.
    LLMClient uses it for requests; BuildAgent uses it for concurrency/cost decisions.
    """

    def __init__(self, config: Optional["Settings"] = None):
        if config is None:
            from src.config.settings import Settings
            config = Settings.load()
        self._api_key = config.api_key()
        self._api_url = config.api_url()
        self._min_context_length = config.model_min_context_length()

        # Caches
        self._models_cache: list[dict] | None = None
        self._key_info_cache: dict | None = None

    @property
    def api_key(self) -> str:
        return self._api_key

    @property
    def api_url(self) -> str:
        return self._api_url

    @property
    def base_url(self) -> str:
        return self._api_url.rsplit("/chat/completions", 1)[0]

    # -- Models API --

    def _fetch_models(self) -> list[dict]:
        """Fetch and cache the full model list from OpenRouter."""
        if self._models_cache is not None:
            return self._models_cache

        import requests
        try:
            resp = requests.get(
                f"{self.base_url}/models",
                headers={"Authorization": f"Bearer {self._api_key}"},
                timeout=15,
            )
            resp.raise_for_status()
            self._models_cache = resp.json().get("data", [])
        except Exception:
            self._models_cache = []
        return self._models_cache or []

    def get_model_info(self, model_id: str) -> dict:
        """Get model info from OpenRouter API. Raises if model not found.

        Returns dict with: context_length, max_completion_tokens, name,
        supported_parameters, pricing, is_free.
        """
        models = self._fetch_models()
        raw = next((m for m in models if m.get("id") == model_id), None)
        if raw is None:
            raise RuntimeError(
                f"Model '{model_id}' not found on OpenRouter. "
                f"Check MEMORY_EXTRACT_MODEL or MEMORY_REASONING_MODEL in .env."
            )

        pricing = raw.get("pricing", {})
        return {
            "context_length": raw.get("context_length", 32_000),
            "max_completion_tokens": raw.get(
                "top_provider", {}
            ).get("max_completion_tokens", 16_384),
            "name": raw.get("name", model_id),
            "supported_parameters": raw.get("supported_parameters", []),
            "pricing": {
                "prompt": float(pricing.get("prompt", "0")),
                "completion": float(pricing.get("completion", "0")),
            },
            "is_free": (
                pricing.get("prompt") == "0"
                and pricing.get("completion") == "0"
            ),
        }

    def get_fallback_info(self, model_id: str) -> dict | None:
        """Like get_model_info but returns None instead of raising."""
        try:
            return self.get_model_info(model_id)
        except RuntimeError:
            return None

    def validate_model(self, model_id: str) -> None:
        """Raise RuntimeError if the model is unsuitable for memory extraction.

        Checks:
          1. Context window >= MIN_CONTEXT_LENGTH
          2. Model supports structured_outputs (json_schema)
        """
        info = self.get_model_info(model_id)
        ctx = info["context_length"]
        if ctx < self._min_context_length:
            raise RuntimeError(
                f"Model '{model_id}' context window ({ctx:,} tokens) is too "
                f"small. Minimum required: {self._min_context_length:,}. "
                f"Use a model with a larger context window."
            )
        supported = info.get("supported_parameters", [])
        if supported and "structured_outputs" not in supported:
            raise RuntimeError(
                f"Model '{model_id}' does not support structured outputs "
                f"(json_schema). Only models with strict JSON schema support "
                f"are allowed. See .env for verified models."
            )

    # -- Key / Rate Limit API --

    def get_key_info(self) -> dict:
        """Query /api/v1/key for account-level rate and usage info.

        Returns dict with: is_free_tier, usage_daily, limit_remaining,
        free_requests_per_minute, free_requests_per_day.
        """
        if self._key_info_cache is not None:
            return self._key_info_cache

        import requests
        try:
            resp = requests.get(
                f"{self.base_url}/key",
                headers={"Authorization": f"Bearer {self._api_key}"},
                timeout=15,
            )
            resp.raise_for_status()
            data = resp.json().get("data", {})
        except Exception:
            # API unreachable — conservative defaults
            data = {"is_free_tier": True}

        is_free_tier = data.get("is_free_tier", True)
        self._key_info_cache = {
            "is_free_tier": is_free_tier,
            "usage_daily": data.get("usage_daily", 0),
            "limit_remaining": data.get("limit_remaining"),
            # OpenRouter docs: free models = 20 req/min
            # Daily: 50 if free tier, 1000 if purchased $10+ credits
            "free_requests_per_minute": 20,
            "free_requests_per_day": 50 if is_free_tier else 1000,
        }
        return self._key_info_cache

    # -- Concurrency --

    def create_rate_limiter(self, model_id: str) -> "RateLimiter":
        """Create a RateLimiter tuned for this model's actual rate limits.

        Detection logic:
          - Free models (pricing $0): 20 RPM (OpenRouter hard limit)
          - Paid models with credits: ~$1 balance = 1 RPS, max 500 RPS
          - Falls back to 20 RPM if detection fails
        """
        model_info = self.get_model_info(model_id)
        key_info = self.get_key_info()
        is_free_model = model_info.get("is_free", True)

        if is_free_model:
            # Free models: hard 20 RPM limit regardless of account tier
            rpm = 20
        else:
            # Paid models: ~$1 balance = 1 RPS, max 500 RPS
            balance = key_info.get("limit_remaining")
            if balance is not None and balance > 0:
                rps = min(balance, 500)
                rpm = int(rps * 60)
            else:
                # No balance info — conservative default
                rpm = 20

        return RateLimiter(rpm=rpm)

    # -- Cost Estimation --

    def estimate_cost(
        self, model_id: str, input_tokens: int, output_tokens: int = 0,
    ) -> float:
        """Estimate cost in USD for a given token count."""
        info = self.get_model_info(model_id)
        pricing = info.get("pricing", {"prompt": 0, "completion": 0})
        return (
            input_tokens * pricing["prompt"]
            + output_tokens * pricing["completion"]
        )


# RateLimiter is re-exported here for backwards compatibility
__all__ = ["OpenRouterAPI", "RateLimiter"]
