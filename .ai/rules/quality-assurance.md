# Quality Assurance

## Scope

- ONLY act on code that exist in .gitmodules

## Initiation

- Apply this checklist only when explicitly asked to "prepare for production" or "review for production."
- State clearly when a rule cannot be verified (e.g., missing tooling) and suggest how to verify.

## Baseline Readiness

- All tests green; new behavior covered by integration-first tests plus targeted unit tests where they add confidence.
- Lint/format clean with project-standard tools (PHPStan/Psalm, ESLint, stylelint, etc.).

## Language-Specific Checks

- PHP: run the project’s configured PHPUnit and static analysis (PHPStan/Psalm) commands—use repo scripts/config, not hard-coded defaults.
- JS/CSS: run the project’s configured test and lint commands; use the repo’s package scripts and configs.

## Code & Design

- Single-responsibility classes/functions; no feature logic parked in UI layers.
- Inputs validated at boundaries; typed interfaces and strict comparisons enforced.
- Remove dead/unused code, feature-flag stubs, and debug scaffolding.

## Security

- Enforce capability + nonce checks (WordPress) or authz middlewares (Node) on state-changing actions.
- Escape/encode output at render; use prepared statements/ORM/repositories for data access.
- Secrets/config only via environment; verify no secrets committed or logged.

## Performance & Stability

- Guard expensive operations with caching/transients where appropriate; avoid N+1 queries.
- Keep external calls wrapped with timeouts/retries; avoid blocking the main request when not required.
- Ensure deterministic tests and predictable time/clock usage (injected clocks).

## Performance Review

- Capture baselines: measure key flows (page render, REST/AJAX endpoints, cron/queue jobs) for time, queries, and memory before/after changes.
- Check database efficiency: no N+1s; indexes used for new queries; paginate large result sets.
- Review assets: keep bundle sizes and critical CSS lean; compress/optimize images; ensure caching headers/versioning are in place.
- Validate caching strategy: object cache/transients for repeat lookups; avoid per-request rewrite flushes; ensure cache keys are scoped and invalidated correctly.
- Assess async/offline work: move heavy tasks to queues/cron where safe; ensure jobs are idempotent and bounded with timeouts/retries.
- Confirm configuration: production settings disable verbose logging/debug; timeouts appropriate for upstream calls.

## Compatibility & Accessibility

- Confirm browser/runtime targets and polyfills (if any) align with project standards.
- CSS uses custom properties/tokens; respects `prefers-reduced-motion`; meets WCAG AA contrast.

## Documentation & Ops

- Update README/PROJECT docs and inline comments where behavior or commands change.
- Note migrations, cron/queue impacts, and new environment variables (names only).
- Provide release notes or a change summary highlighting risks, rollbacks, and manual steps.

## Observability

- Use structured logging; redact sensitive fields.
- Add/align metrics or trace identifiers for new flows where meaningful.
- Ensure error handling paths surface actionable messages without leaking stacks to users.

## Verification Steps (suggest to run)

- Run full test suite and linters.
- Exercise critical user paths manually (happy + edge cases).
- Verify install/activate/uninstall flows for plugins; ensure rewrites not flushed per request.

## Cleanup

- Remove temporary files/fixtures created during testing.
- Ensure git status is clean except for intentional changes; do not leave generated artifacts unless required.
