# Testing Rules (Integration-First TDD)

## Approach
- Practice red → green → refactor: start every change with a failing automated test, then make it pass, then clean up.
- Lead with integration tests that cover observable behavior across boundaries; add unit tests later when they reduce brittleness or speed up feedback.
- Keep tests deterministic: isolate from real networks, clocks, and file systems unless the behavior explicitly depends on them.
- Treat tests as a design tool: if code is hard to test, reshape the production code (interfaces, seams, decomposition) before or alongside adding tests.

## Test Design
- Express test names as behaviors and expected outcomes; include regression references when fixing bugs.
- Cover happy paths, edge cases, and failure handling for new or changed code; security-sensitive flows require explicit negative tests.
- Prefer small, purpose-built fixtures/factories; reset shared state between tests to avoid coupling.

## Tooling
- PHP: favor integration-style suites in PHPUnit; share setup/teardown helpers and avoid global state leakage.
- JavaScript: use the repo-standard test runner (e.g., `npm test` or framework default) with lightweight mocks; keep DOM/WordPress globals faked or sandboxed.
- WordPress: rely on `WP_UnitTestCase` or equivalent integration harnesses; reset hooks, options, and globals after each test.

## Workflow Safeguards
- Do not merge with failing tests; if deferring, mark the test (e.g., `@todo`, `skip`) with a ticket link and rationale.
- Maintain fast feedback: parallelize or split slow suites; quarantine flaky tests and fix them promptly.
- When refactoring without new behavior, keep tests green and prefer improving existing coverage over adding redundant cases.
- Always fix obvious bugs and improve code clarity/safety surfaced by tests; do not ship tests that merely lock in poor behavior.
