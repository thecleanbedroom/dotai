"""Build subsystem — orchestrates commit processing, LLM extraction, and memory creation.

Submodules:
  batching  — token estimation, batch planning, commit splitting
  retry     — LLM call retry with fallback escalation
  factory   — Memory object creation with confidence scoring
  synthesis — incremental synthesis pass against existing corpus
"""

# Re-export BuildAgent so `from src.build import BuildAgent` still works.
# The actual implementation lives in src.build.agent (formerly src/build.py).
from src.build.agent import BuildAgent

__all__ = ["BuildAgent"]
