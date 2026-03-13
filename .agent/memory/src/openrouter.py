"""OpenRouter API client — model info, pricing, rate limits, and validation."""

import fcntl
import json
import pathlib
import sys
import threading
import time
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from src.config import Config


class OpenRouterAPI:
    """Centralized OpenRouter API access — models, pricing, rate limits.

    All model detection, capability checks, and rate info go through this class.
    LLMClient uses it for requests; BuildAgent uses it for concurrency/cost decisions.
    """

    def __init__(self, config: Optional["Config"] = None):
        if config is None:
            from src.config import Config
            config = Config.from_env()
        self._api_key = config.OPENROUTER_API_KEY
        self._api_url = config.MEMORY_BUILD_API_URL
        self._min_context_length = config.MIN_CONTEXT_LENGTH

        # Caches
        self._models_cache: Optional[list[dict]] = None
        self._key_info_cache: Optional[dict] = None

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

    def get_fallback_info(self, model_id: str) -> Optional[dict]:
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
            reason = "free model"
        else:
            # Paid models: ~$1 balance = 1 RPS, max 500 RPS
            balance = key_info.get("limit_remaining")
            if balance is not None and balance > 0:
                rps = min(balance, 500)
                rpm = int(rps * 60)
                reason = f"${balance:.2f} balance"
            else:
                # No balance info — conservative default
                rpm = 20
                reason = "unknown balance"

        print(
            f"  rate limit: {rpm} RPM ({reason})",
            file=sys.stderr, flush=True,
        )
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


class RateLimiter:
    """Cross-process sliding-window rate limiter with 429 emergency braking.

    Uses a shared JSON file with fcntl.flock for atomic access across
    multiple concurrent build processes. All projects sharing the same
    source code (via symlinks) automatically coordinate through the
    same lock file in data/.

    Two layers of protection:
    1. Proactive pacing: acquire() checks a shared sliding window and
       sleeps until a slot opens rather than firing and getting rejected.
    2. Reactive braking: on_rate_limit() writes a cooldown timestamp to
       the shared file, blocking all processes until it expires.

    Thread-safe (threading.Lock) + process-safe (fcntl.flock).
    """

    def __init__(self, rpm: int = 20):
        self._rpm = rpm
        # Shared file in the source data/ dir (follows symlinks = global)
        self._state_file = str(
            pathlib.Path(__file__).resolve().parent.parent / "data" / "rate_limit.json"
        )
        # Thread-level lock (within this process)
        self._thread_lock = threading.Lock()
        # Emergency brake for threads in this process
        self._gate = threading.Event()
        self._gate.set()
        self._brake_lock = threading.Lock()
        self._consecutive_429s = 0
        self._cooldown_until = 0.0
        self._total_429s = 0

    @property
    def rpm(self) -> int:
        return self._rpm

    @property
    def total_429s(self) -> int:
        return self._total_429s

    def _read_state(self, f) -> dict:
        """Read state from locked file."""
        f.seek(0)
        try:
            return json.loads(f.read() or "{}")
        except (json.JSONDecodeError, ValueError):
            return {}

    def _write_state(self, f, state: dict) -> None:
        """Write state to locked file."""
        f.seek(0)
        f.truncate()
        f.write(json.dumps(state))
        f.flush()

    def acquire(self) -> None:
        """Call before each API request. Paces to stay within RPM.

        Coordinates across both threads (threading.Lock) and
        processes (fcntl.flock) using wall-clock timestamps.
        """
        # Wait for local emergency brake
        self._gate.wait()

        while True:
            now = time.time()

            with self._thread_lock:
                try:
                    with open(self._state_file, "r+") as f:
                        fcntl.flock(f, fcntl.LOCK_EX)
                        try:
                            state = self._read_state(f)

                            # Check cross-process cooldown
                            cooldown = state.get("cooldown_until", 0)
                            if cooldown > now:
                                wait = cooldown - now
                                fcntl.flock(f, fcntl.LOCK_UN)
                                time.sleep(wait + 0.1)
                                continue

                            # Sliding window — evict entries older than 60s
                            window = [t for t in state.get("window", [])
                                      if t > now - 60.0]

                            if len(window) < self._rpm:
                                window.append(now)
                                state["window"] = window
                                self._write_state(f, state)
                                return  # Slot acquired

                            # Full — calculate wait
                            sleep_until = window[0] + 60.0
                        finally:
                            fcntl.flock(f, fcntl.LOCK_UN)
                except FileNotFoundError:
                    # First use — create the file and retry
                    pathlib.Path(self._state_file).parent.mkdir(
                        parents=True, exist_ok=True,
                    )
                    with open(self._state_file, "w") as f:
                        f.write("{}")
                    continue

            wait = sleep_until - now
            if wait > 0:
                time.sleep(wait + 0.05)

            self._gate.wait()

    def on_success(self) -> None:
        """Call after a successful response."""
        with self._brake_lock:
            self._consecutive_429s = 0

    def on_rate_limit(self, retry_after: Optional[float] = None) -> None:
        """Call when a 429 is received. Writes cooldown to shared file.

        All processes and threads will block until cooldown expires.
        RPM is NOT reduced — the sliding window already paces correctly,
        and dead processes' timestamps age out in 60s naturally.
        """
        with self._brake_lock:
            self._total_429s += 1
            self._consecutive_429s += 1

            if retry_after and retry_after > 0:
                wait = retry_after
            else:
                wait = min(2 ** self._consecutive_429s, 30)

            cooldown_until = time.time() + wait

            # Write cooldown to shared file so other processes see it
            try:
                with open(self._state_file, "r+") as f:
                    fcntl.flock(f, fcntl.LOCK_EX)
                    try:
                        state = self._read_state(f)
                        if cooldown_until > state.get("cooldown_until", 0):
                            state["cooldown_until"] = cooldown_until
                            self._write_state(f, state)
                    finally:
                        fcntl.flock(f, fcntl.LOCK_UN)
            except FileNotFoundError:
                pass

            self._cooldown_until = cooldown_until
            self._gate.clear()

            print(
                f"      ⏳ 429 #{self._total_429s} — waiting {wait:.0f}s",
                file=sys.stderr, flush=True,
            )

        timer = threading.Timer(wait, self._release)
        timer.daemon = True
        timer.start()

    def _release(self) -> None:
        """Re-open the gate after cooldown."""
        with self._brake_lock:
            if time.time() >= self._cooldown_until:
                self._gate.set()
