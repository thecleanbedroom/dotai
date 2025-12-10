# AI Policy Index

Any AI orchestrator or extension must load and obey **every** document referenced below before proposing changes.

# Environment

- /PROJECT.md – This is the project definition file that explains the project will determine which rule files are appropriate to use.

# Rule Files

## Governance

- /.ai/governance-security.md – Security, privacy, and escalation guidance.

## Languages

- /.ai/language-php.md – PHP rules, formatting guidance, and best practices.

## Platforms

- /.ai/platform-wordpress.md – WordPress-specific instructions shared across projects.

## Tooling & Quality

- /.ai/tooling-testing.md – Required testing + automation steps.
- /.ai/codestyle.json – Machine-readable formatting/lint rules.

## Workflows

- /.ai/workflow-code-review.md – Required review checklist and approvals.

# Repo Policy

## Meta

- /.ai/README.md – Overview of the policy hub concept.
- /.ai/index.md – This manifest (reload whenever the workspace updates).
- /.ai/meta/CONTRIBUTING.md – Workflow for updating policies.
- /.ai/meta/CHANGELOG.md – Historical log of policy edits.
- /.ai/meta/VERSIONING.md – Tagging scheme for policy releases.

## Templates

- /.ai/templates/policy-change.md – Template for introducing or updating policies.
- /.ai/templates/security-issue.md – Template for reporting security concerns.

> _Add any new policy documents or folders in the appropriate section above, then increment the changelog so extensions know a refresh is required._
