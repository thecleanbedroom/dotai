"""Tests for src.llm.client — _extract_json and chat() with mocked HTTP."""

import json

import pytest

from src.llm.client import LLMClient
from src.utils import set_root_dir


class TestExtractJson:
    """Test LLMClient._extract_json static method."""

    def test_pure_json(self):
        raw = '{"new_memories": []}'
        assert LLMClient._extract_json(raw) == raw

    def test_json_with_whitespace(self):
        raw = '  \n  {"key": "val"}  \n  '
        assert LLMClient._extract_json(raw) == '{"key": "val"}'

    def test_fenced_json(self):
        raw = 'Here is the result:\n```json\n{"key": "val"}\n```'
        assert LLMClient._extract_json(raw) == '{"key": "val"}'

    def test_fenced_no_language(self):
        raw = '```\n{"key": "val"}\n```'
        assert LLMClient._extract_json(raw) == '{"key": "val"}'

    def test_dialog_wrapped(self):
        raw = 'Sure, here is the JSON:\n{"key": "val"}\nHope that helps!'
        assert LLMClient._extract_json(raw) == '{"key": "val"}'

    def test_no_json(self):
        raw = "No JSON here at all"
        assert LLMClient._extract_json(raw) == raw

    def test_empty_string(self):
        assert LLMClient._extract_json("") == ""

    def test_nested_json(self):
        nested = '{"outer": {"inner": [1, 2, 3]}}'
        raw = f"Result:\n{nested}\nDone."
        assert LLMClient._extract_json(raw) == nested


class TestTruncateMessage:
    """Test LLMClient._truncate_message."""

    def test_short_message(self):
        msg = {"role": "user", "content": "hello"}
        assert LLMClient._truncate_message(msg) == msg

    def test_long_message(self):
        content = "x" * 1000
        result = LLMClient._truncate_message({"role": "user", "content": content}, max_chars=100)
        assert len(result["content"]) < len(content)
        assert "[1,000 chars total]" in result["content"]
        assert result["role"] == "user"


class TestChat:
    """Test LLMClient.chat() with mocked requests."""

    @pytest.fixture
    def mock_openrouter(self):
        """Minimal OpenRouterAPI mock."""
        class MockOR:
            api_key = "test-key"
            api_url = "https://openrouter.test/v1/chat/completions"
            def get_model_info(self, model):
                return {"name": model, "context_length": 131072, "max_completion_tokens": 65536}
            def validate_model(self, model):
                pass
        return MockOR()

    @pytest.fixture
    def client(self, mock_openrouter, tmp_path, monkeypatch):
        set_root_dir(tmp_path)
        from src.config.settings import Settings
        config = Settings.load()
        return LLMClient(config, model="test/model", openrouter=mock_openrouter)

    def test_chat_success(self, client, monkeypatch):
        """Happy path: valid JSON response."""
        response_data = {
            "choices": [{"message": {"content": '{"result": true}'}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 5,
                      "prompt_tokens_details": {"cached_tokens": 0}},
        }

        class MockResponse:
            status_code = 200
            text = json.dumps(response_data)
            headers: dict = {}  # noqa: RUF012
            def raise_for_status(self):
                pass
            def json(self):
                return response_data

        import requests
        monkeypatch.setattr(requests, "post", lambda *a, **kw: MockResponse())
        result = client.chat([{"role": "user", "content": "test"}], suppress_stats=True)
        parsed = json.loads(result)
        assert parsed["result"] is True

    def test_chat_truncation_raises(self, client, monkeypatch):
        """finish_reason=length should raise ValueError."""
        response_data = {
            "choices": [{"message": {"content": '{"partial": true}'}, "finish_reason": "length"}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 5,
                      "prompt_tokens_details": {"cached_tokens": 0}},
        }

        class MockResponse:
            status_code = 200
            text = json.dumps(response_data)
            headers: dict = {}  # noqa: RUF012
            def raise_for_status(self):
                pass
            def json(self):
                return response_data

        import requests
        monkeypatch.setattr(requests, "post", lambda *a, **kw: MockResponse())
        with pytest.raises(ValueError, match="finish_reason=length"):
            client.chat([{"role": "user", "content": "test"}], suppress_stats=True)

    def test_chat_empty_body_raises(self, client, monkeypatch):
        """Empty 200 response should raise ValueError."""
        class MockResponse:
            status_code = 200
            text = ""
            headers: dict = {}  # noqa: RUF012
            def raise_for_status(self):
                pass
            def json(self):
                return {}

        import requests
        monkeypatch.setattr(requests, "post", lambda *a, **kw: MockResponse())
        with pytest.raises(ValueError, match="empty response body"):
            client.chat([{"role": "user", "content": "test"}], suppress_stats=True)

    def test_chat_no_api_key(self, mock_openrouter, tmp_path, monkeypatch):
        """Missing API key should raise RuntimeError."""
        set_root_dir(tmp_path)
        mock_openrouter.api_key = ""
        from src.config.settings import Settings
        config = Settings.load()
        client = LLMClient(config, model="test/model", openrouter=mock_openrouter)
        with pytest.raises(RuntimeError, match="No API key"):
            client.chat([{"role": "user", "content": "test"}])

    def test_chat_empty_content_raises(self, client, monkeypatch):
        """Empty content with valid finish_reason should raise ValueError."""
        response_data = {
            "choices": [{"message": {"content": ""}, "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 10, "completion_tokens": 0,
                      "prompt_tokens_details": {"cached_tokens": 0}},
        }

        class MockResponse:
            status_code = 200
            text = json.dumps(response_data)
            headers: dict = {}  # noqa: RUF012
            def raise_for_status(self):
                pass
            def json(self):
                return response_data

        import requests
        monkeypatch.setattr(requests, "post", lambda *a, **kw: MockResponse())
        with pytest.raises(ValueError, match="empty content"):
            client.chat([{"role": "user", "content": "test"}], suppress_stats=True)
