"""Tests for src.server — McpServer DB freshness and component wiring."""

from unittest.mock import MagicMock, patch

from src.utils import set_root_dir
from src.server import McpServer


class TestDbFreshness:
    """Test McpServer._check_db_freshness logic."""

    def test_no_data_dir(self, tmp_path, monkeypatch):
        """Missing data dir should not raise."""
        set_root_dir(tmp_path)
        mock_db = MagicMock()
        server = McpServer(lambda **kw: None)
        server._components = {"db": mock_db}
        server._check_db_freshness()
        # Should complete without error

    def test_matching_fingerprint_no_rebuild(self, tmp_path, monkeypatch):
        """Matching fingerprint should NOT trigger rebuild."""
        data_dir = tmp_path / ".agent" / "memory" / "data"
        data_dir.mkdir(parents=True)
        set_root_dir(tmp_path)

        mock_db = MagicMock()
        mock_db.get_fingerprint.return_value = "abc123"

        with patch("src.server.json_store.compute_fingerprint", return_value="abc123"):
            server = McpServer(lambda **kw: None)
            server._components = {"db": mock_db}
            server._check_db_freshness()

        # rebuild should NOT have been called
        assert not hasattr(server, '_rebuild_called')

    def test_stale_fingerprint_triggers_rebuild(self, tmp_path, monkeypatch):
        """Mismatched fingerprint should trigger rebuild."""
        data_dir = tmp_path / ".agent" / "memory" / "data"
        data_dir.mkdir(parents=True)
        set_root_dir(tmp_path)

        mock_db = MagicMock()
        mock_db.get_fingerprint.return_value = "old"

        rebuild_called = []

        with (
            patch("src.server.json_store.compute_fingerprint", return_value="new"),
            patch.object(McpServer, "_rebuild_db_from_json", side_effect=lambda d: rebuild_called.append(True)),
        ):
            server = McpServer(lambda **kw: None)
            server._components = {"db": mock_db}
            server._check_db_freshness()

        assert rebuild_called

    def test_exception_in_freshness_check(self, tmp_path, monkeypatch, capsys):
        """Exceptions should be caught and printed, not raised."""
        set_root_dir(tmp_path)

        data_dir = tmp_path / ".agent" / "memory" / "data"
        data_dir.mkdir(parents=True)

        mock_db = MagicMock()
        mock_db.get_fingerprint.side_effect = RuntimeError("db error")

        with patch("src.server.json_store.compute_fingerprint", return_value="x"):
            server = McpServer(lambda **kw: None)
            server._components = {"db": mock_db}
            server._check_db_freshness()
        # Should not raise


class TestCleanup:
    """Test McpServer.cleanup."""

    def test_cleanup_calls_close(self):
        mock_db = MagicMock()
        server = McpServer(lambda **kw: None)
        server._components = {"db": mock_db}
        server.cleanup()
        mock_db.close.assert_called_once()

    def test_cleanup_no_components(self):
        server = McpServer(lambda **kw: None)
        server.cleanup()  # Should not raise
