"""Tests for Database and schema management."""




class TestDatabase:
    def test_schema_creation(self, db):
        """Schema should create all required tables."""
        tables = db.query(
            "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
        )
        table_names = [r["name"] for r in tables]
        assert "memories" in table_names
        assert "memory_links" in table_names
        assert "build_meta" in table_names
        assert "memories_fts" in table_names

    def test_schema_idempotent(self, db):
        """Calling init_schema twice should not error."""
        db.init_schema()  # Second call
        tables = db.query(
            "SELECT name FROM sqlite_master WHERE type='table'"
        )
        assert len(tables) > 0

    def test_indexes_created(self, db):
        """Schema should create all required indexes."""
        indexes = db.query(
            "SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'"
        )
        idx_names = {r["name"] for r in indexes}
        assert "idx_memories_type" in idx_names
        assert "idx_memories_active" in idx_names
        assert "idx_memories_importance" in idx_names
        assert "idx_memories_confidence" in idx_names
        assert "idx_memory_links_a" in idx_names
        assert "idx_memory_links_b" in idx_names

    def test_fts_triggers(self, db):
        """FTS sync triggers should exist."""
        triggers = db.query(
            "SELECT name FROM sqlite_master WHERE type='trigger'"
        )
        trigger_names = {r["name"] for r in triggers}
        assert "memories_ai" in trigger_names
        assert "memories_ad" in trigger_names
        assert "memories_au" in trigger_names

    def test_drop_all(self, db):
        """drop_all should remove all tables."""
        db.drop_all()
        tables = db.query(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        )
        assert len(tables) == 0
