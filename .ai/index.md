# AI Policy Index

Any AI orchestrator or extension must load and obey **every** document referenced below before proposing changes. At the start of EVERY new conversation, always load all project rules by reading `.ai/index.md` first. If any rule file changes, reload this index and all referenced rules before continuing work.

# Rule Files (all located under `.ai/rules/`)

## Security

- .ai/rules/security.md – Data handling, privacy, and escalation guidance.

## Languages

- .ai/rules/language-php.md – PHP style rules and best practices.
- .ai/rules/language-js.md – Browser/WordPress JavaScript rules (no Node APIs).
- .ai/rules/language-css.md – CSS naming, organization, tokens, and accessibility standards (vanilla CSS only).

## Platforms

- .ai/rules/platform-wordpress.md – WordPress-specific instructions shared across projects.
- .ai/rules/platform-node.md – Node.js runtime, security, and deployment expectations.

## Quality Assurance

- .ai/rules/quality-assurance.md - Rules for preparing project code for production.
- .ai/rules/testing.md - Integration-first TDD testing rules.

## Project

- PROJECT.md - (if present) - Project specific ruleset and environment information. These instructions override any conflicting rules above. Keep this file current when project details change.
