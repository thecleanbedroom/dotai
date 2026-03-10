"""OpenRouter / OpenAI-compatible LLM client."""

import json
import os
from pathlib import Path
from typing import Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from src.config import Config


class LLMClient:
    """Simple OpenAI-compatible API client using requests. No provider SDKs."""

    DEFAULT_API_URL = "https://openrouter.ai/api/v1/chat/completions"
    DEFAULT_MODEL = "anthropic/claude-sonnet-4.6"
    # Minimum context window to be usable for memory extraction.
    # System prompt ~2K + existing memories ~5K + commits + response.
    MIN_CONTEXT_LENGTH = 32_000

    def __init__(
        self,
        config: Optional["Config"] = None,
    ):
        from src import PROJECT_ROOT
        self._log_dir = os.path.join(
            PROJECT_ROOT, ".agent", "memory", "data", "build_responses"
        )
        if config is None:
            from src.config import Config
            config = Config.from_env()
        self.api_key = config.OPENROUTER_API_KEY
        self.api_url = config.MEMORY_BUILD_API_URL
        self.model = config.MEMORY_BUILD_MODEL
        self._min_context_length = config.MIN_CONTEXT_LENGTH
        self._model_info: Optional[dict] = None

    def get_model_info(self) -> dict:
        """Query OpenRouter for model capabilities. Cached after first call.

        Returns dict with keys: context_length, max_completion_tokens, name.
        Falls back to conservative defaults if the API is unreachable.
        """
        if self._model_info is not None:
            return self._model_info

        import requests
        base_url = self.api_url.rsplit("/chat/completions", 1)[0]
        try:
            resp = requests.get(
                f"{base_url}/models",
                headers={"Authorization": f"Bearer {self.api_key}"},
                timeout=15,
            )
            resp.raise_for_status()
            models = resp.json().get("data", [])
            info = next((m for m in models if m.get("id") == self.model), None)
            if info is None:
                raise RuntimeError(
                    f"Model '{self.model}' not found on OpenRouter. "
                    f"Check MEMORY_BUILD_MODEL in .env."
                )

            self._model_info = {
                "context_length": info.get("context_length", 32_000),
                "max_completion_tokens": info.get(
                    "top_provider", {}
                ).get("max_completion_tokens", 16_384),
                "name": info.get("name", self.model),
                "supported_parameters": info.get("supported_parameters", []),
            }
        except RuntimeError:
            raise  # re-raise model-not-found
        except Exception:
            # API unreachable — use conservative defaults
            self._model_info = {
                "context_length": 32_000,
                "max_completion_tokens": 16_384,
                "name": self.model,
                "supported_parameters": [],
            }

        return self._model_info

    def validate_model(self) -> None:
        """Raise RuntimeError if the model is unsuitable for memory extraction.

        Checks:
          1. Context window >= MIN_CONTEXT_LENGTH
          2. Model supports response_format (structured JSON output)
        """
        info = self.get_model_info()
        ctx = info["context_length"]
        if ctx < self._min_context_length:
            raise RuntimeError(
                f"Model '{self.model}' context window ({ctx:,} tokens) is too "
                f"small. Minimum required: {self._min_context_length:,}. "
                f"Use a model with a larger context window."
            )
        supported = info.get("supported_parameters", [])
        if supported and "response_format" not in supported:
            raise RuntimeError(
                f"Model '{self.model}' does not support structured JSON output "
                f"(response_format). Only models with JSON output support are allowed. "
                f"See .env for verified models."
            )

    @staticmethod
    def _strip_json_fences(text: str) -> str:
        """Strip markdown code fences if present (e.g. ```json ... ```)."""
        text = text.strip()
        if text.startswith("```"):
            # Remove opening fence (```json or ```)
            first_newline = text.index("\n") if "\n" in text else len(text)
            text = text[first_newline + 1:]
        if text.endswith("```"):
            text = text[:-3]
        return text.strip()

    def chat(self, messages: list[dict], *, temperature: float = 0.2,
             max_tokens: int = 16384) -> str:
        """Send a chat completion request and return the response content."""
        import requests

        if not self.api_key:
            raise RuntimeError(
                "No API key configured. Set OPENROUTER_API_KEY environment variable."
            )

        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
            "response_format": {"type": "json_object"},
        }

        response = requests.post(
            self.api_url,
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json",
            },
            json=payload,
            timeout=180,
            stream=False,
        )
        response.raise_for_status()

        # Log request + response for debugging
        self._log_exchange(messages, response)

        raw_body = response.text.strip()
        if not raw_body:
            raise ValueError("LLM returned empty response body")

        data = json.loads(raw_body)

        content = data["choices"][0]["message"].get("content") or ""
        finish_reason = data["choices"][0].get("finish_reason", "unknown")

        # Detect truncated output
        if finish_reason == "length":
            raise ValueError(
                f"LLM response truncated (finish_reason=length). "
                f"Reduce batch size or increase max_tokens."
            )

        content = self._strip_json_fences(content)

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

    def _log_exchange(self, messages: list[dict], response) -> None:
        """Save limited request + full response to data/build_responses/.

        Request messages are truncated to keep log files manageable.
        Response uses response.json() because OpenRouter returns SSE-padded
        content even for non-streaming requests.
        """
        try:
            os.makedirs(self._log_dir, exist_ok=True)
            from datetime import datetime
            ts = datetime.now().strftime("%Y%m%d_%H%M%S")
            path = os.path.join(self._log_dir, f"{ts}.json")

            log_entry = {
                "request": {
                    "model": self.model,
                    "messages": [self._truncate_message(m) for m in messages],
                },
                "response": response.json(),
            }
            with open(path, "w") as f:
                json.dump(log_entry, f, indent=2)
        except Exception:
            pass  # Never fail on logging
