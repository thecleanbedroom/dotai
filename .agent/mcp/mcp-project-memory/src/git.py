"""Git log parsing, commit extraction, and trailer handling."""

import re
import subprocess

from src.utils import filter_binary_diffs
from src.memory.models import ParsedCommit


class GitLogParser:
    """Subprocess git log calls, commit parsing, trailer extraction."""

    TRAILER_KEYS = frozenset({
        "type", "rationale", "rejected", "fragile", "related",
        "confidence", "doc-ref",
    })

    def __init__(self, repo_path: str | None = None):
        if repo_path is None:
            from src.utils import root_dir
            repo_path = str(root_dir())
        self.repo_path = repo_path

    def _run_git(self, *args: str) -> str:
        """Run a git command and return stdout."""
        result = subprocess.run(
            ["git", *args],
            capture_output=True, cwd=self.repo_path,
        )
        if result.returncode != 0:
            raise RuntimeError(f"git command failed: {result.stderr.decode(errors='replace').strip()}")
        return result.stdout.decode(errors="replace")

    def parse(self, raw_log: str) -> list[ParsedCommit]:
        """Parse raw git log output into structured commits.

        Splits on NUL byte (\x00) prefix placed at the start of each
        commit's format output.  This ensures each chunk contains the
        complete commit: headers, stat, and diff.
        """
        commits = []
        chunks = raw_log.split("\x00")

        for chunk in chunks:
            chunk = chunk.strip()
            if not chunk:
                continue
            if not any(line.startswith("commit ") for line in chunk.split("\n")):
                continue

            lines = chunk.split("\n")
            header_lines, stat_lines, diff_lines = self._split_sections(lines)

            commit = ParsedCommit()
            self._parse_header(commit, header_lines)
            if not commit.hash:
                continue  # Skip malformed chunks
            self._parse_message_and_trailers(commit, header_lines)
            self._extract_file_list(commit, stat_lines)
            commit.diff = filter_binary_diffs("\n".join(diff_lines))
            commits.append(commit)

        return commits

    @staticmethod
    def _split_sections(
        lines: list[str],
    ) -> tuple[list[str], list[str], list[str]]:
        """Split chunk lines into header, stat, and diff sections."""
        header_lines: list[str] = []
        stat_lines: list[str] = []
        diff_lines: list[str] = []
        in_diff = False
        in_stat = False

        for line in lines:
            if line.startswith("diff --git "):
                in_diff = True
            if in_diff:
                diff_lines.append(line)
            elif re.match(r"^\s+.+\|\s+\d+\s*[+-]*$", line) or re.match(
                r"^\s+\d+ files? changed", line
            ):
                in_stat = True
                stat_lines.append(line)
            else:
                if in_stat:
                    in_stat = False
                header_lines.append(line)

        return header_lines, stat_lines, diff_lines

    @staticmethod
    def _parse_header(commit: ParsedCommit, header_lines: list[str]) -> None:
        """Extract hash, author, and date from header lines."""
        for line in header_lines:
            if line.startswith("commit "):
                commit.hash = line[7:].strip()
            elif line.startswith("Author: "):
                commit.author = line[8:].strip()
            elif line.startswith("Date: "):
                commit.date = line[6:].strip()

    def _parse_message_and_trailers(
        self, commit: ParsedCommit, header_lines: list[str],
    ) -> None:
        """Extract commit message, body, and trailers from header lines."""
        msg_lines: list[str] = []
        trailer_section = False

        for line in header_lines:
            if line.startswith(("commit ", "Author: ", "Date: ")):
                continue
            # Detect trailer lines (Key: Value at end of message)
            if re.match(r"^[A-Za-z][A-Za-z0-9-]*:\s", line) and not trailer_section:
                key = line.split(":")[0].lower()
                if key in self.TRAILER_KEYS:
                    trailer_section = True
            if trailer_section:
                match = re.match(r"^([A-Za-z][A-Za-z0-9-]*):\s*(.*)", line)
                if match:
                    commit.trailers[match.group(1).lower()] = match.group(2).strip()
            else:
                msg_lines.append(line)

        full_msg = "\n".join(msg_lines).strip()
        if full_msg:
            parts = full_msg.split("\n", 1)
            commit.message = parts[0].strip()
            commit.body = parts[1].strip() if len(parts) > 1 else ""

    @staticmethod
    def _extract_file_list(
        commit: ParsedCommit, stat_lines: list[str],
    ) -> None:
        """Extract file paths from git stat lines."""
        for line in stat_lines:
            match = re.match(r"^\s+(.+?)\s+\|", line)
            if match:
                file_path = match.group(1).strip()
                # Handle renames: {old => new}
                if "=>" in file_path:
                    file_path = re.sub(r"\{.*?=>\s*", "", file_path)
                    file_path = file_path.replace("}", "")
                commit.files.append(file_path.strip())

    def get_current_hash(self) -> str:
        """Get the current HEAD commit hash."""
        return self._run_git("rev-parse", "HEAD").strip()

    def get_commit_count(self, *, since_hash: str | None = None) -> int:
        """Count commits, optionally since a given hash."""
        args = ["rev-list", "--count", "HEAD"]
        if since_hash:
            args = ["rev-list", "--count", f"{since_hash}..HEAD"]
        return int(self._run_git(*args).strip())

    def get_all_hashes(self, *, limit: int | None = None) -> list[str]:
        """Return all commit hashes in the repo (newest first).

        Used by the build system to compare against processed.json.
        """
        args = ["log", "--format=%H"]
        if limit:
            args.append(f"-{limit}")
        output = self._run_git(*args).strip()
        if not output:
            return []
        return output.split("\n")

    def get_commits_by_hashes(self, hashes: list[str]) -> list[ParsedCommit]:
        """Fetch full commit data for a list of specific commit hashes.

        One git call per commit — cheap on local disk and eliminates
        all multi-commit parsing ambiguity.
        """
        if not hashes:
            return []

        fmt = "%x00commit %H%nAuthor: %an%nDate: %ai%n%n%B%(trailers)"
        commits = []
        for h in hashes:
            try:
                raw = self._run_git(
                    "log", "--stat", "--patch", f"--format={fmt}",
                    "-1", h,
                )
                commits.extend(self.parse(raw))
            except RuntimeError:
                pass  # Skip missing commits (e.g. after rebase)
        return commits
