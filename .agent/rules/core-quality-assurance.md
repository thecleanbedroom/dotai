# Quality Assurance

## Scope

- ONLY act on folders with `merge.json` (primary) or PROJECT.md "Owned Code Paths" if available at the project root (fallback). Skip vendor/core/contributed plugins.
- Report bugs in unowned code to user with suggested fixes for upstream tickets.

## Initiation

- Apply only when asked to "prepare for production" or "review for production."
- State clearly when a rule cannot be verified and suggest how.

## Baseline

- All tests green; new behavior covered by integration-first tests.
- Lint/format clean with project-standard tools.
- PHP: use repo's PHPUnit/PHPStan config, not hard-coded defaults. Never commit PHPStan stubs into production source.
- JS/CSS: use repo's package scripts and configs.

## Code & Design

- Single-responsibility; no feature logic in UI layers.
- Validate inputs at boundaries; typed interfaces; strict comparisons.
- Remove dead code, feature-flag stubs, debug scaffolding.

## Security & Privacy

- **Data**: treat artifacts as production-bound — no secrets in prompts/logs/files. Redact sensitive values. Add new env vars to `.env` and `.env.example`.
- **Access**: capability/role checks before privileged actions. Anti-forgery tokens on state-changing requests. Prepared statements for all DB access.
- **Output**: escape HTML/attrs/URLs/JSON at render point. Whitelist tags for rich content.
- **Logging**: log security failures. Surface vulnerabilities immediately; pause automation until triaged.
- **AI guardrails**: decline destructive commands unless explicitly instructed. Redact secrets with placeholders. Stop and notify on detected secrets.

## Performance

- Avoid N+1s; use caching/transients; paginate large result sets.
- Wrap external calls with timeouts/retries; avoid blocking main request.
- Keep bundles lean; compress images; ensure cache versioning.
- Validate cache keys scoped and invalidated correctly.
- Move heavy work to queues/cron; ensure idempotent with timeouts.
- _(Suggest to user)_ Capture baselines with profiling tools.
- _(Suggest to user)_ Confirm production disables verbose logging.

## Refactoring

- Update tests, documentation, and evaluate wrapper-vs-replace for existing functions.

## Compatibility & Accessibility

- Confirm browser/runtime targets and polyfills. WCAG AA contrast. `prefers-reduced-motion`.

## Documentation & Ops

- Update README/PROJECT on behavior/command changes. Note migrations, env vars. Provide release notes with risks/rollbacks.

## Observability

- Structured logging; redact sensitive fields. Actionable error messages without stack leaks.
- Add metrics/trace IDs for new flows where meaningful.

## Verification

- Run full test suite and linters.
- _(Suggest to user)_ Exercise critical paths manually. Verify install/activate/uninstall flows.

## Security Sweep

Apply Security & Privacy section above end-to-end, plus: scan recent changes for regressions; add tests for gaps.

## Performance Sweep

Apply Performance section above end-to-end, plus: flag blocking external calls; note findings requiring profiling tools.

## TDD Sweep

Write failing test → make it pass → refactor green. Integration-first per `core-testing.md`. Add unit tests only where they add confidence.

## Cleanup

Remove temp files/fixtures (`.phpunit.cache`, `*.bak`, debug-only output, scratch scripts). Verify no unintended files remain via `git status` (read-only — never stage, commit, or reset).
