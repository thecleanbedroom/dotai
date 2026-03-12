"""Git log parsing, commit extraction, and trailer handling."""

import os
import re
import subprocess
from dataclasses import dataclass, field
from typing import Optional

from src.models import ParsedCommit


class GitLogParser:
    """Subprocess git log calls, commit parsing, trailer extraction."""

    TRAILER_KEYS = frozenset({
        "type", "rationale", "rejected", "fragile", "related",
        "confidence", "doc-ref",
    })

    def __init__(self, repo_path: Optional[str] = None):
        if repo_path is None:
            from src import PROJECT_ROOT
            repo_path = PROJECT_ROOT
        self.repo_path = repo_path

    def _run_git(self, *args: str) -> str:
        """Run a git command and return stdout."""
        result = subprocess.run(
            ["git", *args],
            capture_output=True, text=True, cwd=self.repo_path,
        )
        if result.returncode != 0:
            raise RuntimeError(f"git command failed: {result.stderr.strip()}")
        return result.stdout

    def get_log(
        self,
        *,
        since_hash: Optional[str] = None,
        limit: Optional[int] = None,
    ) -> str:
        """Get git log output with diffs."""
        fmt = "commit %H%nAuthor: %an%nDate: %ai%n%n%B%n%(trailers)%n---END_COMMIT---"
        args = ["log", "--stat", "-p", "--reverse", f"--format={fmt}"]
        if since_hash:
            args.append(f"{since_hash}..HEAD")
        if limit:
            args.append(f"-{limit}")
        return self._run_git(*args)

    def get_file_list(
        self,
        *,
        since_hash: Optional[str] = None,
        limit: Optional[int] = None,
    ) -> str:
        """Get git log with file stats only (lighter weight).

        When limit is set, returns the newest N commits in chronological
        order (git gives newest-first, we reverse).
        """
        fmt = "commit %H%nAuthor: %an%nDate: %ai%n%n%B%n%(trailers)%n---END_COMMIT---"
        args = ["log", "--stat", "--patch", f"--format={fmt}"]
        if not limit:
            args.insert(2, "--reverse")  # all commits: chronological order
        if since_hash:
            args.append(f"{since_hash}..HEAD")
        if limit:
            args.append(f"-{limit}")
        return self._run_git(*args)

    def parse(self, raw_log: str) -> list[ParsedCommit]:
        """Parse raw git log output into structured commits."""
        commits = []
        chunks = raw_log.split("---END_COMMIT---")

        for chunk in chunks:
            chunk = chunk.strip()
            if not chunk:
                continue

            # Quick check: skip if there's no "commit " line in this chunk
            if not any(line.startswith("commit ") for line in chunk.split("\n")):
                continue

            commit = ParsedCommit()

            # Split into header/body and diff sections
            lines = chunk.split("\n")
            in_diff = False
            header_lines = []
            diff_lines = []
            stat_lines = []
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

            # Parse header
            for i, line in enumerate(header_lines):
                if line.startswith("commit "):
                    commit.hash = line[7:].strip()
                elif line.startswith("Author: "):
                    commit.author = line[8:].strip()
                elif line.startswith("Date: "):
                    commit.date = line[6:].strip()

            # Extract message and body (everything after the Date line, before trailers)
            msg_lines = []
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

            # Extract file list from stat lines
            for line in stat_lines:
                match = re.match(r"^\s+(.+?)\s+\|", line)
                if match:
                    file_path = match.group(1).strip()
                    # Handle renames: {old => new}
                    if "=>" in file_path:
                        file_path = re.sub(r"\{.*?=>\s*", "", file_path)
                        file_path = file_path.replace("}", "")
                    commit.files.append(file_path.strip())

            # Filter binary diffs — keep text diffs only
            filtered_diff_lines: list[str] = []
            current_section: list[str] = []
            is_binary = False
            for line in diff_lines:
                if line.startswith("diff --git "):
                    # Flush previous section if not binary
                    if current_section and not is_binary:
                        filtered_diff_lines.extend(current_section)
                    current_section = [line]
                    is_binary = False
                elif "Binary files" in line and "differ" in line:
                    is_binary = True
                    current_section.append(line)
                else:
                    current_section.append(line)
            # Flush last section
            if current_section and not is_binary:
                filtered_diff_lines.extend(current_section)

            commit.diff = "\n".join(filtered_diff_lines)
            commits.append(commit)

        return commits

    def get_current_hash(self) -> str:
        """Get the current HEAD commit hash."""
        return self._run_git("rev-parse", "HEAD").strip()

    def get_commit_count(self, *, since_hash: Optional[str] = None) -> int:
        """Count commits, optionally since a given hash."""
        args = ["rev-list", "--count", "HEAD"]
        if since_hash:
            args = ["rev-list", "--count", f"{since_hash}..HEAD"]
        return int(self._run_git(*args).strip())
