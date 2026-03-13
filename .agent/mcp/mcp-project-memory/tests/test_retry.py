"""Tests for src.build.retry — _classify_error, _extract_retry_after, call_with_retries."""

import json

from src.build.retry import _classify_error, _extract_retry_after, call_with_retries


class TestClassifyError:
    """Test error classification helper."""

    def test_json_decode_error(self):
        e = json.JSONDecodeError("msg", "doc", 0)
        assert _classify_error(e) == (True, False)

    def test_value_error(self):
        assert _classify_error(ValueError("bad")) == (True, False)

    def test_connection_error(self):
        assert _classify_error(ConnectionError("refused")) == (True, False)

    def test_timeout_error(self):
        assert _classify_error(TimeoutError("timed out")) == (True, False)

    def test_os_error(self):
        assert _classify_error(OSError("socket")) == (True, False)

    def test_non_transient(self):
        assert _classify_error(RuntimeError("unknown")) == (False, False)

    def test_key_error(self):
        assert _classify_error(KeyError("missing")) == (False, False)


class TestExtractRetryAfter:
    """Test Retry-After header extraction."""

    def test_no_http_error(self):
        assert _extract_retry_after(ValueError("nope")) is None

    def test_non_numeric_value(self):
        """Non-numeric Retry-After should return None."""
        assert _extract_retry_after(RuntimeError("test")) is None


class TestCallWithRetries:
    """Test call_with_retries with mock LLM."""

    class MockLLM:
        """Configurable mock LLM client."""
        def __init__(self, responses=None, errors=None):
            self._responses = responses or []
            self._errors = errors or []
            self._call_count = 0
            self.model = "test/model"
            self.last_usage = {}

        def chat(self, messages, **kwargs):
            idx = self._call_count
            self._call_count += 1
            if idx < len(self._errors) and self._errors[idx]:
                raise self._errors[idx]
            if idx < len(self._responses):
                return self._responses[idx]
            return '{"result": true}'

        def get_model_info(self):
            return {"name": self.model, "context_length": 131072, "max_completion_tokens": 65536}

    def test_success_first_try(self):
        llm = self.MockLLM(responses=['{"new_memories": []}'])
        result = call_with_retries(
            llm, [{"role": "user", "content": "test"}],
            max_tokens=1000, response_schema={"name": "test", "schema": {}},
        )
        assert result == {"new_memories": []}

    def test_retry_on_json_error(self):
        """JSONDecodeError should trigger retry, then succeed."""
        llm = self.MockLLM(
            responses=["not json", '{"ok": true}'],
            errors=[json.JSONDecodeError("msg", "doc", 0), None],
        )
        result = call_with_retries(
            llm, [{"role": "user", "content": "test"}],
            max_tokens=1000, response_schema={"name": "test", "schema": {}},
            max_retries=3,
        )
        assert result == {"ok": True}

    def test_non_transient_error_immediate_fail(self):
        """Non-transient error should return error dict immediately."""
        llm = self.MockLLM(errors=[RuntimeError("fatal")])
        result = call_with_retries(
            llm, [{"role": "user", "content": "test"}],
            max_tokens=1000, response_schema={"name": "test", "schema": {}},
        )
        assert "error" in result
        assert "fatal" in result["error"]

    def test_truncation_escalation(self):
        """Truncation should escalate to fallback LLM."""
        primary = self.MockLLM(errors=[ValueError("finish_reason=length response truncated")])
        fallback = self.MockLLM(responses=['{"from": "fallback"}'])
        result = call_with_retries(
            primary, [{"role": "user", "content": "test"}],
            max_tokens=1000,
            response_schema={"name": "test", "schema": {}},
            fallback_llm=fallback,
        )
        assert result == {"from": "fallback"}

    def test_truncation_no_fallback(self):
        """Truncation without fallback should return error dict."""
        llm = self.MockLLM(errors=[ValueError("finish_reason=length")])
        result = call_with_retries(
            llm, [{"role": "user", "content": "test"}],
            max_tokens=1000, response_schema={"name": "test", "schema": {}},
        )
        assert "error" in result
        assert "truncated" in result["error"].lower()

    def test_max_retries_exhausted(self):
        """All transient errors should exhaust retries and return error."""
        errors = [ConnectionError("fail")] * 4
        llm = self.MockLLM(errors=errors)
        result = call_with_retries(
            llm, [{"role": "user", "content": "test"}],
            max_tokens=1000, response_schema={"name": "test", "schema": {}},
            max_retries=2,
        )
        assert "error" in result
