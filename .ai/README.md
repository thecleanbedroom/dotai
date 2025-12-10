## DotAI Policy Hub

DotAI centralizes AI usage rules for every VS Code extension in this workspace. Instead of scattering guidance across multiple repos, this directory gathers policy, coding standards, and context information that compatible agents load before acting.

### Purpose

- Provide a single source of truth for AI governance, coding expectations, and environment context.
- Keep Visual Studio Code extensions (Copilot, Cursor, Windsurf, Continue, etc.) aligned so their autofixes and suggestions stay consistent.
- Make it easy to audit or update requirements without editing each plugin’s settings.

### How Extensions Use It

1. On activation, an extension loads the policy manifest and then ingests each referenced rule in order.
2. Policy documents inform completion engines, refactoring tools, and chat-based helpers so they follow the same standards.
3. When policies change, only this directory needs a version bump; extensions automatically inherit the update the next time they refresh context.

### Maintaining the Hub

- Keep instructions generic and product-agnostic so they apply across workspaces sharing these VS Code plugins.
- Add or remove rules exclusively through the manifest so agents always discover them automatically.
- Prefer concise, action-oriented language so extensions can parse rules quickly.
- Document breaking changes in git commits or workspace notes so plugin maintainers know to reload policies.

### Contributor Checklist

- [ ] Verify new guidance does not conflict with existing rules; edit or deprecate outdated guidance as needed.
- [ ] Ensure the manifest reflects any additions, renames, or removals.
- [ ] Keep examples minimal and generic to avoid leaking project-specific details.
- [ ] Test policy ingestion with at least one VS Code extension to ensure it reads the full manifest without errors.
