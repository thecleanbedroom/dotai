## DotAI Policy Hub

DotAI centralizes AI usage rules for every VS Code extension in this workspace. Instead of scattering guidance across multiple repos, the `.ai` directory gathers policy, coding standards, and context files that any compatible agent or plugin must load before acting.

### Purpose

- Provide a single source of truth for AI governance, coding expectations, and environment context.
- Keep Visual Studio Code extensions (Copilot, Cursor, Windsurf, Continue, etc.) aligned so their autofixes and suggestions stay consistent.
- Make it easy to audit or update requirements without editing each plugin’s settings.

### Directory Structure

- `index.md` – manifest every extension must load before acting.
- `context.md` – shared environment assumptions (WordPress, Lando, tooling).
- `language-*.md` – language-specific rule bundles (e.g., `language-php-psr12.md`).
- `platform-*.md` – platform playbooks such as WordPress guidance.
- `governance-*.md` – security/privacy escalation procedures.
- `tooling-*.md` – testing, linting, and machine-readable style expectations.
- `workflow-*.md` – human + agent workflows (code review, release, incident response).
- `adapter-*.md` – extension-specific ingestion instructions.
- `templates/` – reusable issue/PR templates for policy changes and incidents.
- `meta/` – contributor instructions, changelog, and versioning policy.

### How Extensions Use It

1. On activation, an extension reads `index.md` and then loads each referenced file in order.
2. Policy documents inform completion engines, refactoring tools, and chat-based helpers so they follow the same standards.
3. When policies change, only the `.ai` directory needs a version bump; extensions automatically inherit the update the next time they refresh context.

### Maintaining the Hub

- Keep instructions generic and product-agnostic so they apply across workspaces sharing these VS Code plugins.
- When adding a new policy file, register it in `index.md` so agents discover it automatically.
- Prefer concise, action-oriented language so extensions can parse rules quickly.
- Document breaking changes in git commits or workspace notes so plugin maintainers know to reload policies.

### Contributor Checklist

- [ ] Verify new guidance does not conflict with existing files; edit or deprecate outdated rules.
- [ ] Update `index.md` whenever you add, rename, or remove policy documents.
- [ ] Keep examples minimal and generic to avoid leaking project-specific details.
- [ ] Test policy ingestion with at least one VS Code extension to ensure it reads the full index without errors.
