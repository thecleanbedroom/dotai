# AI Agent Plugin Configs

This directory contains configuration files for various AI coding assistants. Each subdirectory mirrors the dot-directory structure expected at the project root.

## Available Plugins

| Directory    | Tool           | Config File               |
| ------------ | -------------- | ------------------------- |
| `.codex/`    | OpenAI Codex   | `config.toml`             |
| `.continue/` | Continue       | `rules/rules.md`          |
| `.cursor/`   | Cursor         | `rules/policy.mdc`        |
| `.gemini/`   | Gemini         | `styleguide.md`           |
| `.github/`   | GitHub Copilot | `copilot-instructions.md` |
| `.windsurf/` | Windsurf       | `rules/rules.md`          |

## Usage

To enable a plugin for your project, copy its directory to your project root:

```bash
# Example: enable Cursor support
cp -r .agent/plugins/.cursor /path/to/project/

# Example: enable all plugins
cp -r .agent/plugins/.* /path/to/project/
```

## Syncing

All plugin configs reference `.agent/index.md` as the shared source of truth. When the agent system changes, regenerate plugin configs from the index:

1. Update `.agent/index.md` with current rules and conventions
2. Copy the updated content into each plugin's config file, adapted to its format
3. Copy the updated plugin directories to the project root

The `/audit-all` workflow checks whether plugin configs are in sync with the index.
