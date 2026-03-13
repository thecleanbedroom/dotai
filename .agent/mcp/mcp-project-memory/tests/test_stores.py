"""Tests for MemoryStore, LinkStore, and BuildMetaStore."""


from src.memory.models import BuildMetaEntry, Memory, MemoryLink


class TestMemoryStore:
    def test_create_and_get(self, memory_store):
        """Should create a memory and retrieve it by ID."""
        mem = memory_store.create(Memory(
            summary="Test memory",
            type="decision",
            confidence=75,
            importance=80,
            file_paths=["src/foo.py"],
        ))
        assert mem.id is not None
        assert len(mem.id) == 36  # UUID format
        fetched = memory_store.get(mem.id)
        assert fetched.summary == "Test memory"
        assert fetched.type == "decision"
        assert fetched.confidence == 75
        assert fetched.importance == 80
        assert fetched.file_paths == ["src/foo.py"]
        assert fetched.active is True

    def test_update(self, memory_store):
        """Should update an existing memory."""
        mem = memory_store.create(Memory(summary="Original", type="context"))
        mem.summary = "Updated"
        mem.importance = 90
        memory_store.update(mem)
        fetched = memory_store.get(mem.id)
        assert fetched.summary == "Updated"
        assert fetched.importance == 90

    def test_deactivate(self, memory_store):
        """Should soft-delete a memory."""
        mem = memory_store.create(Memory(summary="To deactivate", type="debt"))
        memory_store.deactivate(mem.id)
        fetched = memory_store.get(mem.id)
        assert fetched.active is False

    def test_touch(self, memory_store):
        """Should increment access_count and update accessed_at."""
        mem = memory_store.create(Memory(summary="Touchable", type="context"))
        assert mem.access_count == 0
        memory_store.touch(mem.id)
        memory_store.touch(mem.id)
        fetched = memory_store.get(mem.id)
        assert fetched.access_count == 2

    def test_query_by_file_exact(self, memory_store):
        """Should find memories associated with a specific file."""
        memory_store.create(Memory(
            summary="Foo service", type="pattern", file_paths=["src/services/foo.py"],
        ))
        memory_store.create(Memory(
            summary="Bar service", type="pattern", file_paths=["src/services/bar.py"],
        ))
        results = memory_store.query_by_file("src/services/foo.py")
        assert len(results) == 1
        assert results[0].summary == "Foo service"

    def test_query_by_file_directory(self, memory_store):
        """Should find memories matching a directory prefix."""
        memory_store.create(Memory(
            summary="Service A", type="pattern", file_paths=["src/services/a.py"],
        ))
        memory_store.create(Memory(
            summary="Service B", type="pattern", file_paths=["src/services/b.py"],
        ))
        memory_store.create(Memory(
            summary="Model C", type="pattern", file_paths=["src/models/c.py"],
        ))
        results = memory_store.query_by_file("src/services/")
        assert len(results) == 2

    def test_query_by_file_min_importance(self, memory_store):
        """Should filter by minimum importance."""
        memory_store.create(Memory(
            summary="Important", type="decision", importance=90,
            file_paths=["src/app.py"],
        ))
        memory_store.create(Memory(
            summary="Unimportant", type="context", importance=10,
            file_paths=["src/app.py"],
        ))
        results = memory_store.query_by_file("src/app.py", min_importance=50)
        assert len(results) == 1
        assert results[0].summary == "Important"

    def test_search_fts(self, memory_store):
        """FTS5 search should find memories by content."""
        memory_store.create(Memory(
            summary="Webhook architecture uses handler classes", type="decision",
        ))
        memory_store.create(Memory(
            summary="Database migrations run sequentially", type="convention",
        ))
        results = memory_store.search("webhook handler")
        assert len(results) >= 1
        assert "webhook" in results[0].summary.lower() or "handler" in results[0].summary.lower()

    def test_search_with_type_filter(self, memory_store):
        """FTS search should filter by memory type."""
        memory_store.create(Memory(
            summary="Auth uses JWT tokens", type="decision",
        ))
        memory_store.create(Memory(
            summary="Auth middleware validates JWT", type="pattern",
        ))
        results = memory_store.search("JWT", memory_type="decision")
        assert len(results) == 1
        assert results[0].type == "decision"

    def test_search_prefix_matching(self, memory_store):
        """Prefix matching: 'audit' should match 'auditing'."""
        memory_store.create(Memory(
            summary="Auditing workflow runs nightly",
            type="convention",
        ))
        results = memory_store.search("audit")
        assert len(results) >= 1
        assert "audit" in results[0].summary.lower()

    def test_search_special_chars_safe(self, memory_store):
        """Queries with special chars should not crash FTS5."""
        memory_store.create(Memory(summary="Test memory", type="context"))
        # These should not raise exceptions
        results = memory_store.search('test (with) "quotes"')
        assert isinstance(results, list)
        results = memory_store.search("NEAR/3 stuff")
        assert isinstance(results, list)

    def test_search_by_type_in_fts(self, memory_store):
        """Searching for 'decision' should find decision-type memories via FTS."""
        memory_store.create(Memory(
            summary="Chose SQLite over Postgres",
            type="decision",
        ))
        memory_store.create(Memory(
            summary="Renamed function for clarity",
            type="context",
        ))
        results = memory_store.search("decision")
        assert len(results) >= 1
        assert any(r.type == "decision" for r in results)

    def test_tags_create_and_retrieve(self, memory_store):
        """Tags should round-trip through create/get."""
        m = memory_store.create(Memory(
            summary="Added webhook handler",
            type="pattern",
            tags=["webhook", "api", "handler"],
        ))
        retrieved = memory_store.get(m.id)
        assert retrieved is not None
        assert retrieved.tags == ["webhook", "api", "handler"]

    def test_search_by_tag(self, memory_store):
        """Searching for a tag keyword should match via FTS."""
        memory_store.create(Memory(
            summary="Implemented caching layer",
            type="pattern",
            tags=["caching", "performance", "redis"],
        ))
        memory_store.create(Memory(
            summary="Added logging middleware",
            type="convention",
            tags=["logging", "middleware"],
        ))
        results = memory_store.search("redis")
        assert len(results) >= 1
        assert "caching" in results[0].summary.lower()

    def test_search_match_any(self, memory_store):
        """match='any' should return results matching any term (OR)."""
        memory_store.create(Memory(
            summary="Webhook handler refactored",
            type="pattern",
            tags=["webhook"],
        ))
        memory_store.create(Memory(
            summary="Database migration added",
            type="convention",
            tags=["database"],
        ))
        # match="all" (AND) — both terms must match, likely 0 results
        results_and = memory_store.search("webhook database", match="all")
        # match="any" (OR) — either term matches
        results_or = memory_store.search("webhook database", match="any")
        assert len(results_or) >= 2
        assert len(results_or) > len(results_and)

    def test_count(self, memory_store):
        """Should count active and total memories."""
        memory_store.create(Memory(summary="A", type="context"))
        memory_store.create(Memory(summary="B", type="context"))
        m3 = memory_store.create(Memory(summary="C", type="context"))
        memory_store.deactivate(m3.id)
        assert memory_store.count(active_only=True) == 2
        assert memory_store.count(active_only=False) == 3

    def test_stats(self, memory_store):
        """Should return aggregate statistics."""
        memory_store.create(Memory(
            summary="A", type="decision", confidence=75, importance=80,
            file_paths=["src/a.py"],
        ))
        memory_store.create(Memory(
            summary="B", type="pattern", confidence=40, importance=60,
            file_paths=["src/a.py", "src/b.py"],
        ))
        stats = memory_store.stats()
        assert stats["total_memories"] == 2
        assert stats["by_type"]["decision"] == 1
        assert stats["by_type"]["pattern"] == 1
        assert stats["confidence"]["avg"] > 0
        assert stats["confidence"]["min"] == 40
        assert stats["confidence"]["max"] == 75
        assert stats["top_files"]["src/a.py"] == 2

    def test_get_nonexistent(self, memory_store):
        """Should return None for nonexistent ID."""
        assert memory_store.get("nonexistent-uuid") is None

    def test_list_all(self, memory_store):
        """Should list all memories sorted by importance."""
        memory_store.create(Memory(summary="Low", type="context", importance=20))
        memory_store.create(Memory(summary="High", type="context", importance=90))
        results = memory_store.list_all()
        assert len(results) == 2
        assert results[0].importance > results[1].importance


class TestLinkStore:
    def test_create_and_get(self, memory_store, link_store):
        """Should create a link and retrieve it."""
        m1 = memory_store.create(Memory(summary="A", type="decision"))
        m2 = memory_store.create(Memory(summary="B", type="pattern"))
        link = link_store.create(MemoryLink(
            memory_id_a=m1.id, memory_id_b=m2.id,
            relationship="related_to", strength=0.7,
        ))
        assert link.id is not None
        links = link_store.get_links_for(m1.id)
        assert len(links) == 1
        assert links[0].relationship == "related_to"

    def test_bidirectional(self, memory_store, link_store):
        """Links should be retrievable from either side."""
        m1 = memory_store.create(Memory(summary="A", type="decision"))
        m2 = memory_store.create(Memory(summary="B", type="pattern"))
        link_store.create(MemoryLink(
            memory_id_a=m1.id, memory_id_b=m2.id,
            relationship="caused_by",
        ))
        # Query from m2's side
        links = link_store.get_links_for(m2.id)
        assert len(links) == 1

    def test_get_linked_ids(self, memory_store, link_store):
        """Should return IDs of linked memories."""
        m1 = memory_store.create(Memory(summary="A", type="decision"))
        m2 = memory_store.create(Memory(summary="B", type="pattern"))
        m3 = memory_store.create(Memory(summary="C", type="context"))
        link_store.create(MemoryLink(
            memory_id_a=m1.id, memory_id_b=m2.id, relationship="related_to",
        ))
        link_store.create(MemoryLink(
            memory_id_a=m1.id, memory_id_b=m3.id, relationship="caused_by",
        ))
        linked = link_store.get_linked_ids(m1.id)
        assert set(linked) == {m2.id, m3.id}

    def test_delete_for_memory(self, memory_store, link_store):
        """Should delete all links involving a memory."""
        m1 = memory_store.create(Memory(summary="A", type="decision"))
        m2 = memory_store.create(Memory(summary="B", type="pattern"))
        link_store.create(MemoryLink(
            memory_id_a=m1.id, memory_id_b=m2.id, relationship="related_to",
        ))
        link_store.delete_for_memory(m1.id)
        assert len(link_store.get_links_for(m1.id)) == 0
        assert len(link_store.get_links_for(m2.id)) == 0


class TestBuildMetaStore:
    def test_record_and_get(self, build_meta_store):
        """Should record a build and retrieve the last one."""
        entry = build_meta_store.record(BuildMetaEntry(
            build_type="incremental",
            commit_count=10, memory_count=5,
        ))
        assert entry.id is not None
        last = build_meta_store.get_last()
        assert last.build_type == "incremental"
        assert last.commit_count == 10

    def test_get_last_returns_newest(self, build_meta_store):
        """Should return the most recent build."""
        build_meta_store.record(BuildMetaEntry(
            build_type="full", commit_count=5, memory_count=2,
        ))
        build_meta_store.record(BuildMetaEntry(
            build_type="incremental", commit_count=3, memory_count=4,
        ))
        last = build_meta_store.get_last()
        assert last.build_type == "incremental"
        assert last.commit_count == 3

    def test_get_last_empty(self, build_meta_store):
        """Should return None when no builds exist."""
        assert build_meta_store.get_last() is None
