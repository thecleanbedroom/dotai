"""Memory factory — creates Memory objects from LLM output with confidence scoring.

Extracted from BuildAgent to satisfy Single Responsibility:
BuildAgent orchestrates; MemoryFactory handles Memory construction.
"""

from src.memory.models import Memory


class MemoryFactory:
    """Creates Memory objects from LLM extraction output.

    Confidence is computed from evidence signals weighted by
    discriminative value:
      - source_commits: 0→0, 1→8, 2→20, 3+→30     (max 30, structural)
      - files:          0→0, 1→5, 2-3→15, 4-6→25, 7+→30 (max 30, structural)
      - summary length: ≤100→0, ≤200→5, ≤300→12, 300+→20 (max 20, cosmetic)
      - tags:           ≤2→0, 3-4→5, 5-6→12, 7+→20      (max 20, cosmetic)
    """

    @staticmethod
    def from_llm_output(data: dict) -> Memory:
        """Convert a dict from LLM output into a Memory dataclass."""
        source_commits = data.get("source_commits", [])
        files = data.get("files", [])
        summary = data.get("summary", "")
        tags = data.get("tags", [])

        score = 0

        n_commits = len(source_commits)
        if n_commits >= 3:
            score += 30
        elif n_commits == 2:
            score += 20
        elif n_commits == 1:
            score += 8

        n_files = len(files)
        if n_files >= 7:
            score += 30
        elif n_files >= 4:
            score += 25
        elif n_files >= 2:
            score += 15
        elif n_files == 1:
            score += 5

        s_len = len(summary)
        if s_len > 300:
            score += 20
        elif s_len > 200:
            score += 12
        elif s_len > 100:
            score += 5

        n_tags = len(tags)
        if n_tags >= 7:
            score += 20
        elif n_tags >= 5:
            score += 12
        elif n_tags >= 3:
            score += 5

        return Memory(
            summary=summary,
            type=data.get("type", "context"),
            confidence=score,
            importance=data.get("importance", 50),
            source_commits=source_commits,
            file_paths=files,
            tags=tags,
            created_at=data.get("created_at", ""),
        )
