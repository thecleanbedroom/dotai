# AI Policy Index

Any AI orchestrator or extension must load and obey **every** document referenced below before proposing changes.

## Loading Requirements

1. At the start of EVERY new conversation, read this file (`.ai/index.md`) first.
2. **Load ALL rule files listed below**—do not skip any. Read each file in full.
3. If a rule file references other files, load those recursively.
4. If any rule file changes mid-session, reload this index and all referenced rules.
5. If a user prompt conflicts with these rules, stop and ask for guidance before acting.

# Rule Files (all located under `.ai/rules/`)

## Prompt Aliases

- .ai/rules/prompt-aliases.md - Shortcuts for common multi-step requests (e.g., code sweep, QA review, TDD cycle).

## Workflow

- .ai/rules/workflow.md - Workflow orchestration rules.

## Security

- .ai/rules/security.md – Data handling, privacy, and escalation guidance.

## Languages

- .ai/rules/language-default.md - Language agnostic rules and best practices.
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
