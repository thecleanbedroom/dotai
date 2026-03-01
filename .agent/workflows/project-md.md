---
description: Generate or regenerate PROJECT.md by analyzing the codebase and following its instructions
---

# Generate or Regenerate PROJECT.md

PROJECT.md is an **orientation document** — it tells a new engineer or AI agent what this project is, how it's structured, and how to get started. Think README, not reference manual.

## Hard Requirements

- Do not include coding rules, workflow conventions, or style guides — those belong in `.agent/rules/`, not here.
- Inspect the repo to discover project facts. Do not guess. If a fact is not verifiable, label it "Unknown" or "TBD".
- Do not include secrets. If you find committed secrets (API keys, passwords, DSNs, webhooks):
  - State that a secret exists (without the value).
  - Identify where it is using a relative repo path.
  - Recommend rotation and migration to environment-based secrets management.
- Do not include any links: no web URLs, no `file://` URIs, no absolute paths. Use plain relative repo paths (e.g., [.lando.yml], [laravel/config/database.php]) and placeholders (e.g., `<local-site-domain>`).
- Output only the final contents of [PROJECT.md] in Markdown.

## What to Include

Create a concise orientation document with these sections (omit only if truly not applicable):

1. **Project Overview** — what this repo is and key constraints
2. **Architecture / Code Layout** — key directories, entry points, autoloading conventions
3. **Development Environment** — local tooling, runtime versions, how to start/stop, known-good workflows
4. **Commands** — essential workflows only (setup, test, deploy), point to the task runner for the full list
5. **Testing & Quality** — how to run tests/linters, config file paths, important overrides
6. **Platform-specific notes** — only if applicable (WordPress multisite, Node build conventions, etc.)
7. **Logging, Debugging, Observability** — log locations, debug tools, error reporting posture
8. **Deployment** — tooling, environments, critical env vars (names only, reference the env example file for the full list)
9. **Security & Data Handling** — access controls, secrets management, notable footguns
10. **Do / Don't Quick Rules** — short repo-specific rules (commands to use, paths to avoid, common mistakes)

## Steps

1. **Discover the project stack.** Check for and read whichever of these exist:
   - Container/environment config (Docker Compose, Lando, Vagrant, etc.)
   - Dockerfiles or build scripts
   - Task runners and their help output (Makefile, justfile, npm scripts, etc.)
   - Dependency manifests (composer.json, package.json, etc.)
   - Environment contracts (`.env.example`, etc.)

2. **Discover the application.** The stack tells you the platform — now find what makes this app _this app_. Look past framework scaffolding for the custom code: modules, plugins, themes, extensions, services, packages, apps. When you find a container directory (e.g., a modules folder, a plugins folder, an extensions folder), list **all siblings** inside it. Browse each subsystem enough to understand its purpose. Nothing should be silently omitted from the final document.

3. **Gather context from knowledge and history:**
   - Review knowledge items and memories for architecture decisions, migrations, and known gotchas.
   - Check recent conversation history for relevant context about the project's evolution.
   - List `docs/finished/` (if it exists) and scan titles for project timeline.

4. **Generate PROJECT.md** following the hard requirements and section guide above. Additional guidance:
   - **Orientation, not reference.** Someone should understand the project and how to start, not get a help manual.
   - **Don't duplicate tool output.** Point to the task runner for full command lists.
   - **Don't duplicate env files.** Reference the example file, highlight only critical vars.
   - **Cover every subsystem** found in step 2.
   - **Highlight non-obvious conventions.** Call out anything where a reasonable person's first guess would be wrong: application code nested in a subdirectory instead of the repo root, namespace prefixes that don't match the directory structure intuitively, config that lives in an unexpected location (e.g., inside modules instead of the framework root), entry points that aren't where the framework docs say they'd be.
   - **Show how subsystems connect.** If the project has multiple subsystems, include a brief description of how they relate — which one sends data to which, and via what mechanism (REST API, queue, shared DB, etc.).
   - **Clarify repo boundaries.** Note what's owned by this repo vs what lives elsewhere (companion apps, external APIs, separately-deployed services). Prevents searching for code that isn't here.
   - **Note what requires a restart.** If some changes take effect immediately and others need a service restart, cache clear, or rebuild, say so. This is a universal source of confusion.

5. **Write** to [PROJECT.md] in the repo root (overwrite), then present for review.
