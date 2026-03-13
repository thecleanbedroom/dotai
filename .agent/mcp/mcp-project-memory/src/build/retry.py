"""LLM call retry handler — retry logic, transient error detection, fallback escalation.

Extracted from BuildAgent to satisfy Single Responsibility:
BuildAgent orchestrates; RetryHandler manages resilience.
"""

import json
import sys
import threading
import time
from typing import Optional

from src.llm.client import LLMClient
from src.llm.rate_limiter import RateLimiter
from src.config.internal import InternalSettings


def _classify_error(e: Exception) -> tuple[bool, bool]:
    """Classify an exception as (is_transient, is_rate_limit)."""
    if isinstance(e, (json.JSONDecodeError, ValueError)):
        return True, False
    if isinstance(e, (ConnectionError, TimeoutError, OSError)):
        return True, False
    try:
        from requests.exceptions import HTTPError
        if isinstance(e, HTTPError) and e.response is not None:
            code = e.response.status_code
            if code == 429:
                return True, True
            if code >= 500:
                return True, False
    except ImportError:
        pass
    return False, False


def _extract_retry_after(e: Exception) -> float | None:
    """Extract Retry-After header value from an HTTPError, if present."""
    try:
        from requests.exceptions import HTTPError
        if isinstance(e, HTTPError) and e.response is not None:
            val = e.response.headers.get("Retry-After")
            if val:
                return float(val)
    except (ImportError, ValueError):
        pass
    return None


def call_with_retries(
    llm: LLMClient, messages: list[dict],
    *, max_tokens: int, response_schema: dict,
    max_retries: int = InternalSettings.retry_max_retries(),
    fallback_llm: LLMClient | None = None,
    label: str = "",
    print_lock: Optional["threading.Lock"] = None,
    rate_limiter: RateLimiter | None = None,
) -> dict | None:
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
                suppress_stats=True,
            )
            if rate_limiter:
                rate_limiter.on_success()
            return json.loads(response_text)
        except Exception as e:
            last_error = e

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
                    max_tokens = fb_max
                    continue
                return {"error": (
                    f"Output truncated (model output cap hit). "
                    f"Use a model with a higher max_completion_tokens. {e}"
                )}

            is_transient, is_rate_limit = _classify_error(e)

            if not is_transient or attempt >= max_retries - 1:
                return {"error": f"call failed: {e}"}

            if is_rate_limit and rate_limiter:
                rate_limiter.on_rate_limit(_extract_retry_after(e))
                continue

            wait = InternalSettings.retry_rate_limit_base_wait() * (2 ** attempt) if is_rate_limit else InternalSettings.retry_transient_base_wait() + attempt
            print(
                f"      retry {attempt + 1}/{max_retries - 1} after {wait}s ({e})",
                file=sys.stderr, flush=True,
            )
            time.sleep(wait)
            continue
    return {"error": f"call failed after {max_retries} attempts: {last_error}"}
