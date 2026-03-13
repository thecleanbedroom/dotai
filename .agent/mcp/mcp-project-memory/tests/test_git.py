"""Tests for GitLogParser."""

from src.git import GitLogParser


class TestGitLogParser:
    def test_parse_simple_commit(self):
        """Should parse a basic commit with message and files."""
        raw = "\x00commit abc12345\nAuthor: Test User\nDate: 2026-03-09 10:00:00 -0500\n\nFix login bug\n\n src/auth.py | 5 ++---\n 1 file changed, 2 insertions(+), 3 deletions(-)"
        parser = GitLogParser()
        commits = parser.parse(raw)
        assert len(commits) == 1
        assert commits[0].hash == "abc12345"
        assert commits[0].author == "Test User"
        assert commits[0].message == "Fix login bug"
        assert "src/auth.py" in commits[0].files

    def test_parse_commit_with_trailers(self):
        """Should extract git trailers from commit messages."""
        raw = "\x00commit def67890\nAuthor: Test User\nDate: 2026-03-09 10:00:00 -0500\n\nAdd webhook handler pattern\n\nImplemented the handler class pattern for webhooks.\nEach handler type gets its own class.\n\nType: feature\nRationale: Keeps handler logic isolated and testable\nConfidence: high\n\n src/webhooks/handler.py | 50 ++++++++++++++++++++\n 1 file changed, 50 insertions(+)"
        parser = GitLogParser()
        commits = parser.parse(raw)
        assert len(commits) == 1
        c = commits[0]
        assert c.message == "Add webhook handler pattern"
        assert "handler class pattern" in c.body
        assert c.trailers.get("type") == "feature"
        assert c.trailers.get("rationale") == "Keeps handler logic isolated and testable"
        assert c.trailers.get("confidence") == "high"

    def test_parse_multiple_commits(self):
        """Should parse multiple commits from a single log output."""
        raw = (
            "\x00commit aaa111\nAuthor: Dev A\nDate: 2026-03-01 09:00:00 -0500\n\n"
            "First commit\n\n README.md | 1 +\n 1 file changed, 1 insertion(+)\n"
            "\x00commit bbb222\nAuthor: Dev B\nDate: 2026-03-02 10:00:00 -0500\n\n"
            "Second commit\n\n src/app.py | 10 ++++++++++\n 1 file changed, 10 insertions(+)"
        )
        parser = GitLogParser()
        commits = parser.parse(raw)
        assert len(commits) == 2
        assert commits[0].hash == "aaa111"
        assert commits[1].hash == "bbb222"

    def test_parse_bare_commit(self):
        """Should handle minimal 'hotfix' style commits."""
        raw = "\x00commit ccc333\nAuthor: Dev\nDate: 2026-03-09 10:00:00 -0500\n\nhotfix\n\n src/fix.py | 1 +\n 1 file changed, 1 insertion(+)"
        parser = GitLogParser()
        commits = parser.parse(raw)
        assert len(commits) == 1
        assert commits[0].message == "hotfix"
        assert commits[0].body == ""
        assert len(commits[0].trailers) == 0

    def test_parse_rename(self):
        """Should handle file renames in stat output."""
        raw = "\x00commit ddd444\nAuthor: Dev\nDate: 2026-03-09 10:00:00 -0500\n\nRename file\n\n {old => new}/file.py | 0\n 1 file changed"
        parser = GitLogParser()
        commits = parser.parse(raw)
        assert len(commits) == 1
        # Rename should produce a cleaned-up path
        assert len(commits[0].files) >= 1
