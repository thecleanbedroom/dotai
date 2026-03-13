"""Tests for OpenRouterAPI — model info, validation, rate limiting, cost estimation."""

from unittest.mock import MagicMock, patch

import pytest

from src.config.settings import Settings
from src.llm.openrouter import OpenRouterAPI
from src.llm.rate_limiter import RateLimiter


@pytest.fixture()
def api():
    settings = Settings(
        openrouter_api_key="sk-test",
        memory_build_api_url="https://openrouter.ai/api/v1/chat/completions",
        min_context_length=32_000,
    )
    return OpenRouterAPI(settings)


# -- _fetch_models --

class TestFetchModels:
    def test_returns_cached(self, api):
        api._models_cache = [{"id": "m1"}]
        result = api._fetch_models()
        assert result == [{"id": "m1"}]

    @patch("requests.get")
    def test_fetches_from_api(self, mock_get, api):
        mock_resp = MagicMock()
        mock_resp.json.return_value = {"data": [{"id": "m1"}, {"id": "m2"}]}
        mock_get.return_value = mock_resp
        result = api._fetch_models()
        assert len(result) == 2
        assert api._models_cache is not None
        mock_get.assert_called_once()

    @patch("requests.get")
    def test_api_error_returns_empty(self, mock_get, api):
        mock_get.side_effect = Exception("timeout")
        result = api._fetch_models()
        assert result == []


# -- get_model_info --

class TestGetModelInfo:
    def test_returns_structured_info(self, api):
        api._models_cache = [{
            "id": "test/model",
            "name": "Test Model",
            "context_length": 128_000,
            "top_provider": {"max_completion_tokens": 32_000},
            "supported_parameters": ["structured_outputs"],
            "pricing": {"prompt": "0.001", "completion": "0.002"},
        }]
        info = api.get_model_info("test/model")
        assert info["context_length"] == 128_000
        assert info["max_completion_tokens"] == 32_000
        assert info["name"] == "Test Model"
        assert info["is_free"] is False
        assert info["pricing"]["prompt"] == 0.001

    def test_free_model_detected(self, api):
        api._models_cache = [{
            "id": "free/model",
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        info = api.get_model_info("free/model")
        assert info["is_free"] is True

    def test_model_not_found_raises(self, api):
        api._models_cache = []
        with pytest.raises(RuntimeError, match="not found"):
            api.get_model_info("nonexistent/model")


# -- get_fallback_info --

class TestGetFallbackInfo:
    def test_returns_none_on_missing(self, api):
        api._models_cache = []
        assert api.get_fallback_info("missing/model") is None

    def test_returns_info_on_found(self, api):
        api._models_cache = [{"id": "m1", "pricing": {"prompt": "0", "completion": "0"}}]
        info = api.get_fallback_info("m1")
        assert info is not None
        assert info["is_free"] is True


# -- validate_model --

class TestValidateModel:
    def test_passes_valid_model(self, api):
        api._models_cache = [{
            "id": "valid/model",
            "context_length": 128_000,
            "supported_parameters": ["structured_outputs"],
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        api.validate_model("valid/model")  # should not raise

    def test_rejects_small_context(self, api):
        api._models_cache = [{
            "id": "small/model",
            "context_length": 4_000,
            "supported_parameters": ["structured_outputs"],
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        with pytest.raises(RuntimeError, match="too small"):
            api.validate_model("small/model")

    def test_rejects_no_structured_outputs(self, api):
        api._models_cache = [{
            "id": "noschema/model",
            "context_length": 128_000,
            "supported_parameters": ["temperature"],
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        with pytest.raises(RuntimeError, match="structured outputs"):
            api.validate_model("noschema/model")

    def test_allows_empty_supported_params(self, api):
        """When supported_parameters is empty, skip the check (OpenRouter may omit)."""
        api._models_cache = [{
            "id": "unknown/model",
            "context_length": 128_000,
            "supported_parameters": [],
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        api.validate_model("unknown/model")  # should not raise


# -- get_key_info --

class TestGetKeyInfo:
    @patch("requests.get")
    def test_returns_key_info(self, mock_get, api):
        mock_resp = MagicMock()
        mock_resp.json.return_value = {
            "data": {"is_free_tier": False, "usage_daily": 0.5, "limit_remaining": 10.0}
        }
        mock_get.return_value = mock_resp
        info = api.get_key_info()
        assert info["is_free_tier"] is False
        assert info["limit_remaining"] == 10.0

    @patch("requests.get")
    def test_caches_result(self, mock_get, api):
        mock_resp = MagicMock()
        mock_resp.json.return_value = {"data": {"is_free_tier": True}}
        mock_get.return_value = mock_resp
        api.get_key_info()
        api.get_key_info()
        mock_get.assert_called_once()

    @patch("requests.get")
    def test_api_error_defaults_conservative(self, mock_get, api):
        mock_get.side_effect = Exception("timeout")
        info = api.get_key_info()
        assert info["is_free_tier"] is True


# -- create_rate_limiter --

class TestCreateRateLimiter:
    def test_free_model_gets_20rpm(self, api):
        api._models_cache = [{
            "id": "free/model",
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        api._key_info_cache = {"is_free_tier": True}
        limiter = api.create_rate_limiter("free/model")
        assert isinstance(limiter, RateLimiter)
        assert limiter.rpm == 20

    def test_paid_model_scales_with_balance(self, api):
        api._models_cache = [{
            "id": "paid/model",
            "pricing": {"prompt": "0.001", "completion": "0.002"},
        }]
        api._key_info_cache = {"is_free_tier": False, "limit_remaining": 5.0}
        limiter = api.create_rate_limiter("paid/model")
        assert limiter.rpm == 5 * 60  # $5 → 5 RPS → 300 RPM

    def test_paid_model_caps_at_500rps(self, api):
        api._models_cache = [{
            "id": "paid/model",
            "pricing": {"prompt": "0.001", "completion": "0.002"},
        }]
        api._key_info_cache = {"is_free_tier": False, "limit_remaining": 1000.0}
        limiter = api.create_rate_limiter("paid/model")
        assert limiter.rpm == 500 * 60  # capped at 500 RPS

    def test_paid_model_unknown_balance(self, api):
        api._models_cache = [{
            "id": "paid/model",
            "pricing": {"prompt": "0.001", "completion": "0.002"},
        }]
        api._key_info_cache = {"is_free_tier": False, "limit_remaining": None}
        limiter = api.create_rate_limiter("paid/model")
        assert limiter.rpm == 20  # conservative default


# -- estimate_cost --

class TestEstimateCost:
    def test_zero_for_free_model(self, api):
        api._models_cache = [{
            "id": "free/model",
            "pricing": {"prompt": "0", "completion": "0"},
        }]
        cost = api.estimate_cost("free/model", input_tokens=100_000)
        assert cost == 0.0

    def test_computes_cost(self, api):
        api._models_cache = [{
            "id": "paid/model",
            "pricing": {"prompt": "0.001", "completion": "0.002"},
        }]
        cost = api.estimate_cost("paid/model", input_tokens=1000, output_tokens=500)
        assert cost == pytest.approx(1000 * 0.001 + 500 * 0.002)


# -- Properties --

class TestProperties:
    def test_api_key(self, api):
        assert api.api_key == "sk-test"

    def test_api_url(self, api):
        assert api.api_url == "https://openrouter.ai/api/v1/chat/completions"

    def test_base_url(self, api):
        assert api.base_url == "https://openrouter.ai/api/v1"
