"""LLM chat client — handles request/response, retries, logging."""

import json
import os
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from src.config.settings import Settings
    from src.llm.openrouter import OpenRouterAPI


class LLMClient:
    """Chat completion client. Uses OpenRouterAPI for model/provider concerns."""

    def __init__(
        self,
        config: Optional["Settings"] = None,
        *,
        model: str | None = None,
        openrouter: Optional["OpenRouterAPI"] = None,
    ):
        from src.utils import data_dir
        self._log_dir = str(data_dir() / "build")
        if config is None:
            from src.config.settings import Settings
            config = Settings.load()
        if openrouter is None:
            from src.llm.openrouter import OpenRouterAPI
            openrouter = OpenRouterAPI(config)
        self._openrouter = openrouter
        self.model = model or config.extraction_model()
        self.api_key = openrouter.api_key
        self.api_url = openrouter.api_url

    def get_model_info(self) -> dict:
        """Delegate to OpenRouterAPI for model capabilities."""
        return self._openrouter.get_model_info(self.model)

    def validate_model(self) -> None:
        """Delegate to OpenRouterAPI for model validation."""
        self._openrouter.validate_model(self.model)

    @staticmethod
    def _extract_json(text: str) -> str:
        """Extract JSON object from LLM response that may contain surrounding text.

        Handles: pure JSON, ```json fences, dialog text before/after JSON.
        """
        text = text.strip()

        # 1. Try parsing directly — pure JSON
        if text.startswith("{"):
            return text

        # 2. Extract from markdown code fences (```json ... ``` or ``` ... ```)
        import re
        fence_match = re.search(r"```(?:json)?\s*\n(.*?)```", text, re.DOTALL)
        if fence_match:
            return fence_match.group(1).strip()

        # 3. Last resort: find first { to last } — handles dialog-wrapped JSON
        first_brace = text.find("{")
        last_brace = text.rfind("}")
        if first_brace != -1 and last_brace > first_brace:
            return text[first_brace:last_brace + 1]

        return text

    def chat(self, messages: list[dict], *, temperature: float = 0.2,
             max_tokens: int = 16384,
             response_schema: dict | None = None,
             label: str = "",
             print_lock: object | None = None,
             suppress_stats: bool = False) -> str:
        """Send a chat completion request and return the response content.

        Args:
            response_schema: Optional JSON schema dict for strict structured output.
                If provided, uses json_schema mode with strict: true.
                If not provided, uses json_object mode.
        """
        import time as _time

        import requests

        if not self.api_key:
            raise RuntimeError(
                "No API key configured. Set OPENROUTER_API_KEY environment variable."
            )

        # Build response_format based on whether a schema is provided
        if response_schema:
            response_format = {
                "type": "json_schema",
                "json_schema": {
                    "name": response_schema.get("name", "response"),
                    "strict": True,
                    "schema": response_schema["schema"],
                },
            }
        else:
            response_format = {"type": "json_object"}

        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
            "response_format": response_format,
            # Ensure provider actually supports structured output
            "provider": {"require_parameters": True},
            # Enable prompt caching via OpenRouter.
            "cache_control": {"type": "ephemeral"},
        }

        t0 = _time.monotonic()
        response = requests.post(
            self.api_url,
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json",
                # Required by OpenRouter for app identification.
                # Absence can cause empty responses.
                "HTTP-Referer": "http://localhost",
                "X-Title": "project-memory",
            },
            json=payload,
            timeout=180,
            stream=False,
        )
        elapsed = _time.monotonic() - t0

        # Log diagnostics for non-200 responses before raise_for_status
        if response.status_code != 200:
            import sys
            body_snippet = response.text[:200] if response.text else "(empty)"
            print(
                f"    API error {response.status_code}: {body_snippet}",
                file=sys.stderr, flush=True,
            )

        # Always log the exchange (even failures) for diagnostics
        self._log_exchange(payload, response)

        response.raise_for_status()

        raw_body = response.text.strip()
        if not raw_body:
            raise ValueError("LLM returned empty response body (200 with no content)")

        data = json.loads(raw_body)

        # Print timing and usage stats
        usage = data.get("usage", {})
        prompt_tokens = usage.get("prompt_tokens", 0)
        completion_tokens = usage.get("completion_tokens", 0)
        cached = usage.get("prompt_tokens_details", {}).get("cached_tokens", 0)

        # Store for callers that want to include tokens in their own output
        self.last_usage = {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "cached_tokens": cached,
            "elapsed": round(elapsed, 1),
        }

        if not suppress_stats:
            import sys
            prefix = f"    {label}: " if label else "    "
            stats_line = (
                f"{prefix}{elapsed:.1f}s | "
                f"in: {prompt_tokens:,} (cached: {cached:,}) | "
                f"out: {completion_tokens:,}"
            )
            if print_lock:
                with print_lock:
                    print(stats_line, file=sys.stderr, flush=True)
            else:
                print(stats_line, file=sys.stderr, flush=True)

        content = data["choices"][0]["message"].get("content") or ""
        finish_reason = data["choices"][0].get("finish_reason", "unknown")

        # Detect truncated output
        if finish_reason == "length":
            raise ValueError(
                "LLM response truncated (finish_reason=length). "
                "Reduce batch size or increase max_tokens."
            )

        content = self._extract_json(content)

        # Detect empty content (transient API issue)
        if not content.strip():
            raise ValueError(
                f"LLM returned empty content (finish_reason={finish_reason}). "
                f"This is a transient API issue — will retry."
            )

        return content

    @staticmethod
    def _truncate_message(msg: dict, max_chars: int = 500) -> dict:
        """Return a message dict with content truncated for logging."""
        content = msg.get("content", "")
        if len(content) <= max_chars:
            return msg
        return {
            **msg,
            "content": content[:max_chars] + f"... [{len(content):,} chars total]",
        }

    def _log_exchange(self, payload: dict, response) -> None:
        """Save limited request + raw response to data/build_responses/.

        Logs BEFORE parsing so failures are captured too.
        Failed responses get an _error_ prefix for easy identification.
        """
        try:
            os.makedirs(self._log_dir, exist_ok=True)
            from datetime import datetime
            ts = datetime.now().strftime("%Y%m%d_%H%M%S")

            is_error = response.status_code != 200 or not response.text.strip()
            prefix = "_error_" if is_error else ""
            path = os.path.join(self._log_dir, f"{prefix}{ts}.json")

            # Parse response JSON if possible, fall back to raw text
            try:
                resp_data = response.json()
            except Exception:
                resp_data = {
                    "raw_text": response.text[:2000] if response.text else "(empty)",
                    "status_code": response.status_code,
                    "headers": dict(response.headers),
                }

            log_entry = {
                "request": {
                    "model": self.model,
                    "max_tokens": payload.get("max_tokens"),
                    "response_format": payload.get("response_format", {}).get("type"),
                    "messages": [self._truncate_message(m) for m in payload.get("messages", [])],
                },
                "response": resp_data,
            }
            with open(path, "w") as f:
                json.dump(log_entry, f, indent=2)
        except Exception:
            pass  # Never fail on logging
