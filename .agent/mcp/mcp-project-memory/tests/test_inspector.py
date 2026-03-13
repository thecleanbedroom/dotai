"""Tests for Inspector."""

import json

from src.memory.models import BuildMetaEntry, Memory


class TestInspector:
    def test_help(self, inspector):
        """Help should list all available commands."""
        result = inspector.inspect("help")
        assert "tables" in result
        assert "memories" in result
        assert "links" in result
        assert "builds" in result
        assert "stats" in result
        assert "schema" in result
        assert "fts" in result

    def test_tables(self, inspector):
        """Tables command should list all database tables."""
        result = json.loads(inspector.inspect("tables"))
        names = [t["name"] for t in result]
        assert "memories" in names
        assert "memory_links" in names
        assert "build_meta" in names

    def test_memories_empty(self, inspector):
        """Memories command should return empty list when no memories exist."""
        result = json.loads(inspector.inspect("memories"))
        assert result == []

    def test_memories_with_data(self, inspector, stores):
        """Memories command should return all memories."""
        stores["memory_store"].create(Memory(summary="Test", type="context"))
        result = json.loads(inspector.inspect("memories"))
        assert len(result) == 1
        assert result[0]["summary"] == "Test"

    def test_single_memory(self, inspector, stores):
        """memory <id> should return a specific memory with links."""
        mem = stores["memory_store"].create(Memory(summary="Specific", type="decision"))
        result = json.loads(inspector.inspect(f"memory {mem.id}"))
        assert result["memory"]["summary"] == "Specific"
        assert "links" in result

    def test_single_memory_not_found(self, inspector):
        """memory <id> for nonexistent ID should return error."""
        result = inspector.inspect("memory nonexistent-uuid")
        assert "not found" in result

    def test_links_empty(self, inspector):
        """Links command should return empty list."""
        result = json.loads(inspector.inspect("links"))
        assert result == []

    def test_builds_empty(self, inspector):
        """Builds command should return empty list."""
        result = json.loads(inspector.inspect("builds"))
        assert result == []

    def test_builds_with_data(self, inspector, stores):
        """Builds command should show build history."""
        stores["build_meta_store"].record(BuildMetaEntry(
            build_type="full", commit_count=10, memory_count=5,
        ))
        result = json.loads(inspector.inspect("builds"))
        assert len(result) == 1
        assert result[0]["build_type"] == "full"
        assert result[0]["commit_count"] == 10

    def test_stats_empty(self, inspector):
        """Stats command should work on empty store."""
        result = json.loads(inspector.inspect("stats"))
        assert result["total_memories"] == 0
        assert result["last_build"] is None

    def test_schema(self, inspector):
        """Schema command should show CREATE statements."""
        result = inspector.inspect("schema")
        assert "CREATE TABLE" in result
        assert "memories" in result
        assert "memory_links" in result

    def test_fts_health(self, inspector, stores):
        """FTS health should report sync status."""
        stores["memory_store"].create(Memory(summary="FTS test", type="context"))
        result = json.loads(inspector.inspect("fts"))
        assert result["in_sync"] is True
        assert result["fts_rows"] == 1
        assert result["memory_rows"] == 1

    def test_unknown_command(self, inspector):
        """Unknown command should show help."""
        result = inspector.inspect("banana")
        assert "Unknown" in result
        assert "tables" in result

    def test_empty_query(self, inspector):
        """Empty query should show help."""
        result = inspector.inspect("")
        assert "tables" in result
