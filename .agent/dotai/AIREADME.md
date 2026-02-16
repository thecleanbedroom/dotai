# AI Plugin Propagation

`.agent/index.md` is the single source of truth for bootstrap instructions. Its content must be copied to each AI plugin's config location so every tool gets the same pre-flight rules.

## Targets

| Plugin     | Target Path                       | Format   | Notes                                     |
| ---------- | --------------------------------- | -------- | ----------------------------------------- |
| Gemini     | `.gemini/styleguide.md`           | markdown | Copy verbatim                             |
| Copilot    | `.github/copilot-instructions.md` | markdown | Copy verbatim                             |
| Windsurf   | `.windsurf/rules/rules.md`        | markdown | Copy verbatim                             |
| Continue   | `.continue/rules/rules.md`        | markdown | Copy verbatim                             |
| Cursor     | `.cursor/rules/policy.mdc`        | mdc      | Wrap with YAML frontmatter (see below)    |
| Codex      | `.codex/config.toml`              | TOML     | Escape into `instructions = "..."` string |
| Cursor alt | `.cursorrules`                    | markdown | Copy verbatim (legacy location)           |

## Quick sync

For markdown targets, copy directly:

```bash
for f in .gemini/styleguide.md .github/copilot-instructions.md .windsurf/rules/rules.md .continue/rules/rules.md .cursorrules; do
  cp .agent/index.md "$f"
done
```

Cursor `.mdc` needs frontmatter prepended:

```bash
printf -- '---\ndescription: "AI Policy: Read .agent/index.md for project rules and coding standards"\nalwaysApply: true\n---\n\n' > .cursor/rules/policy.mdc
cat .agent/index.md >> .cursor/rules/policy.mdc
```

Codex TOML requires the content escaped as a single-line string in `instructions = "..."`. Update `.codex/config.toml` manually or regenerate it.

## When to sync

Re-run propagation whenever `.agent/index.md` is modified. The `/audit-agent` workflow (Q29) checks for drift between `index.md` and all targets.

## AGENTS.sh behavior

`AGENTS.sh` syncs the dotai policy repo into the project root. It **excludes** `.github` and `.agent/skills` but **includes** all other plugin directories. After a fresh `AGENTS.sh` sync, plugin configs will be up to date from the repo. Manual propagation is only needed when editing `index.md` locally.
