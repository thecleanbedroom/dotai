# Testing Rules (Integration-First TDD)

## Approach

- Red → green → refactor for behavioral changes. Comment/doc-only fixes may skip test-first.
- Integration tests first (observable behavior across boundaries); unit tests later for speed/confidence.
- Deterministic: isolate from real networks, clocks, filesystems. Use injected clocks for time-sensitive logic.
- Code hard to test? Reshape production code (interfaces, seams, decomposition).

## Development Phases

- During iterative design exploration with the user, focus on production code only. Do not fix tests after each incremental change — the design is still in flux and test fixes will be reworked.
- Defer all test updates until the user approves the design direction.
- Once approved, fix all tests in a single pass before marking work as done.

## Test Design

- Names express behaviors and outcomes; include bug/regression refs.
- Cover happy paths, edge cases, failure handling. Security-sensitive flows require negative tests.
- Small, purpose-built fixtures; reset shared state between tests.
- Every test method must contain at least one framework assertion. For delegation tests that only verify mock expectations, explicitly increment the assertion count rather than relying on mock teardown.

## Tooling

- PHP: PHPUnit integration style; shared setup/teardown; no global state leakage.
- JS: repo-standard runner (`npm test`); lightweight mocks; sandboxed DOM/globals.
- WordPress: `WP_UnitTestCase`; reset hooks/options/globals per test.

## Safeguards

- No merging with failing tests. Mark deferred tests with `@todo`/`skip` + ticket link.
- Fast feedback: parallelize/split slow suites; quarantine flaky tests.
- Refactoring without new behavior: keep green, improve existing coverage.
- Fix bugs surfaced by tests; don't lock in poor behavior.
- Never weaken a production interface (e.g., making a parameter nullable) just because test mocks return null. Fix the test to provide a proper mock — tests must adapt to the interface, not the other way around.
- Coverage: ~80% overall, ~100% on new/changed code, 100% on security-critical paths. Meaningful assertions over coverage numbers.

## External Dependency Isolation

- When adding or changing an external dependency (Redis, database, API), immediately check the test base class. If tests share the production resource, add isolation (separate DB, mock, or config override) in the same change.
