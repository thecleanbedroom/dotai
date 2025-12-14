# PROJECT.md Generation Prompt

You are generating [PROJECT.md] for this repository. This file is part of the [.ai/index.md] policy set and its instructions override other `.ai/rules/*` documents if there is a conflict.

## Hard requirements
- Inspect the repo to discover project facts. Do not guess. If a fact is not verifiable from the repo, label it as “Unknown” or “TBD”.
- Do not include secrets. If you discover secrets committed in the repo (API keys, passwords, DSNs, webhooks), DO NOT repeat them in [PROJECT.md]. Instead:
  - state that a secret exists (without the value),
  - identify where it is using a relative repo path and a brief description of where in the file,
  - recommend rotation and migration to environment-based secrets management.
- Keep it practical: the output must be usable onboarding/reference for engineers and future AI agents.
- Output only the final contents of [PROJECT.md] in Markdown (no extra commentary).
- Do not include any links of any kind:
  - no web URLs,
  - no `file://` URIs,
  - no absolute filesystem paths.
  Use plain relative repo paths (examples: [.lando.yml], [web/wp-config.php]) and plain text placeholders (example: `<local-site-domain>`).

## What to include in PROJECT.md
Create a concise but detailed document with sections like these (omit only if truly not applicable):

1. Project Overview
   - What this repo is (WordPress multisite, theme/plugin repo, Node service, etc.)
   - Key constraints (multisite, subdomain install, headless, etc.)

2. Architecture / Code Layout (repo-specific)
   - Key directories and what lives there
   - Entry points (plugin bootstrap files, theme entry files, service entry)
   - Autoloading/module conventions discovered (Composer, PSR-4, bundlers)

3. Development Environment
   - Local tooling (Lando/Docker/etc.), runtime versions discovered (PHP/Node)
   - How to start/stop services using repo-standard commands
   - Local domains / proxy config described using placeholders (example: `<local-site-domain>`)
   - Known-good workflows

4. Commands (authoritative, repo-specific)
   - Exact commands engineers should run, preferring repo wrappers (examples: `lando wp ...`, `lando phpcs`, `composer ...`, `npm ...`)
   - If deploy tooling exists, document it and include an explicit safety note (example: do not run deploy commands unless explicitly instructed)

5. Testing & Quality
   - How to run linters/formatters/tests from this repo
   - Reference config files by relative path
   - Any important overrides or non-standard conventions discovered

6. Platform-specific notes (only if applicable)
   - WordPress: multisite mode and key constants (from config), domain mapping/sunrise notes, WP-CLI usage patterns, caching/object-cache considerations
   - Node: build output folder conventions, environment variables (names only), runtime expectations

7. Logging, Debugging, Observability
   - Where logs go in local and production (use relative paths like `logs/<name>.log`)
   - Any code/config that redirects logging (plugins, constants, ini settings)
   - Debug/profiling tools and how to enable/disable them via repo commands
   - Error reporting posture (dev vs prod) without exposing secrets

8. Deployment
   - What tool is used (from repo files), where the config lives (relative path)
   - Environments/stages discovered
   - Required environment variables (names only)

9. Security & Data Handling (repo-specific addendum)
   - Any platform-required controls (capabilities/nonces/escaping for WP, etc.)
   - How secrets are expected to be managed in this project
   - Notable “footguns” found in config (described without secrets)

10. Do / Don’t Quick Rules
   - Short repo-specific rules based on what you found (commands to use, paths to avoid, common mistakes)

Now generate the complete [PROJECT.md] for this repo.
