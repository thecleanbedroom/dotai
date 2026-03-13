"""Shared test fixtures for the project memory system."""

import os
import sys
import tempfile

import pytest

# Route __pycache__ into .venv
os.environ.setdefault("PYTHONPYCACHEPREFIX", os.path.join(os.path.dirname(__file__), "..", ".venv", "pycache"))

# Add parent directory to path so src/ package is importable
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from src.db import Database
from src.inspector import Inspector
from src.memory.stores import BuildMetaStore, LinkStore, MemoryStore


@pytest.fixture
def db():
    """Create a temporary database for testing."""
    with tempfile.NamedTemporaryFile(suffix=".db", delete=False) as f:
        db_path = f.name
    database = Database(db_path)
    # Hold connection open for the test session — avoids per-call open/close overhead
    database.hold()
    yield database
    database.release()
    os.unlink(db_path)


@pytest.fixture
def memory_store(db):
    return MemoryStore(db)


@pytest.fixture
def link_store(db):
    return LinkStore(db)


@pytest.fixture
def build_meta_store(db):
    return BuildMetaStore(db)




@pytest.fixture
def components(db):
    return {
        "db": db,
        "memory_store": MemoryStore(db),
        "link_store": LinkStore(db),
        "build_meta_store": BuildMetaStore(db),
    }


@pytest.fixture
def stores(db):
    return {
        "memory_store": MemoryStore(db),
        "link_store": LinkStore(db),
        "build_meta_store": BuildMetaStore(db),
    }


@pytest.fixture
def inspector(db, stores):
    return Inspector(db, stores["memory_store"], stores["link_store"],
                     stores["build_meta_store"])
