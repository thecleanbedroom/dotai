---
description: "Canonical code smell checklist — used standalone for targeted scans or referenced by /plan, /implement, /sweep for continuous observation"
---

# /sniff — Code Smell Detection

Canonical checklist for identifying code smells, structural issues, and quality concerns. Used two ways:

**Standalone** — `/sniff path/to/area` — targeted scan of a specific area, outputs findings to a debt doc
**Embedded** — referenced by `/plan`, `/implement`, `/sweep`, and `/testcoverage` for continuous observation while working

> [!IMPORTANT]
> Sniff is **eyes-only observation**. You notice things while reading code — you don't run commands, tools, or analysis. For active scanning (running tests, coverage, linters), use `/sweep`.

### Smell checklist

Apply these checks to every file you read or touch. In embedded mode, observe passively and log findings. In standalone mode, scan systematically.

---

#### Structural smells

**SRP violations** — God classes, files doing too many things, methods with 5+ responsibilities. Look for classes with 10+ public methods or files over 500 lines that aren't data/config. Also: overlapping responsibilities between classes/modules — grep for a domain term, if it's mutated in 2+ packages, ownership is unclear.

**DRY violations** — Duplicated logic across files, copy-pasted code with minor variations, parallel hierarchies that could be unified. Near-identical methods in sibling classes.

**Naming** — Files, classes, or methods that don't match what they do. Inconsistent naming conventions within the same area (mixed camelCase/snake_case). Names that would make a newcomer guess wrong.

**Misplaced code** — Business logic in controllers/views/routes, framework code in domain layer, config in the wrong directory, utilities that belong in a shared location. Directories with 10+ files that should be grouped by concern.

**Dead code & orphaned definitions** — Unused functions, unreachable branches, stale imports, commented-out code. Also: exported constants, validation maps, or type aliases defined but never referenced by any consumer. Definitions that look like they should enforce something but don't.

**Organization** — Missing test files for production code, inconsistent directory structure across similar modules (e.g., `users/` has `service.go` + `handler.go` but `orders/` inlines both in one file), deeply nested hierarchies that could be flattened.

**Interface violations** — Public methods that should be private, missing type hints, overly permissive visibility, nullable parameters that aren't genuinely optional, leaky abstractions.

**Magic values** — Hardcoded strings, numbers, URLs, model identifiers, endpoint paths, or protocol headers that should be constants or config values. Repeated literal values across files.

**Implicit dependencies** — Function that requires reading 3+ other files to understand its inputs, outputs, or side effects. Undocumented setup order requirements. Global state that must be initialized before a module works.

**Standards gaps** — Deviations from the project's own rules (`.agent/rules/`, linter config, style guides, `CONTRIBUTING.md`), framework best practices, or widely-accepted principles (SOLID, clean architecture, etc.).

---

#### Security smells

**Hardcoded secrets** — API keys, passwords, tokens, or credentials in source code instead of environment variables.

**Unvalidated input** — User input or external API responses consumed via type assertions or blind property access without existence checks. Missing type checks on external data. Deeply nested optional chains on untyped responses.

**Unsafe output** — HTML/attributes/URLs rendered without escaping. User content echoed directly.

**Raw SQL** — String concatenation in queries instead of prepared statements or parameterized queries.

**Missing auth checks** — Privileged actions without capability/role verification. Missing anti-forgery tokens on state-changing requests.

---

#### Performance smells

**N+1 queries** — Query inside a loop, or loading related records one at a time instead of batch/eager loading.

**Blocking calls in request path** — External HTTP calls, heavy computation, or file I/O in the main request without timeout/async.

**Unbounded results** — Queries or collections without pagination or limits. Also: user-generated content sent to external services without size validation — prompts, payloads, or messages assembled from user data without length guards.

**Repeated expensive work** — Same computation or query executed multiple times without caching. Cache keys that aren't scoped or invalidated correctly.

**Missing timeouts** — External calls without timeout configuration. Queue jobs without timeout alignment.

---

#### Lifecycle & state smells

**Ephemeral state in restartable processes** — Module-level mutable variables in processes that can be terminated and restarted (workers, serverless, daemons). State should be persisted, re-derivable, or explicitly documented as ephemeral.

**Connection singletons without recovery** — Cached database or service connections without disconnect/error handlers that reset the singleton when the underlying connection drops.

**Unguarded state transitions** — Control actions (pause/resume/cancel) that don't verify current state before mutating. Operations should be no-ops or errors when invoked from invalid source states.

**Leaking control flags** — Boolean flags (paused, cancelled, locked) set by one operation but not cleared before the next operation reads them. Previous operation state bleeding into new operations.

---

#### Content & degradation smells

**Silent fallback on edge-case input** — Functions that return plausible-looking but incorrect results for empty, oversized, or unusually-encoded input rather than clearly erroring or logging.

**Silent no-ops** — Actions that do nothing when preconditions fail (e.g., `if (!ready) return`). In user-facing code: disable the control or show feedback. In backend code: log at warning level or return an explicit error explaining why the action was skipped.

**Raw platform errors** — Storage quota, network, or permission errors surfaced to callers as unprocessed runtime messages instead of actionable guidance. In user-facing code, translate to human-readable messages. In libraries/APIs, wrap with domain-specific error types.

**Lost error metadata** — Error objects re-thrown or wrapped in ways that strip custom properties (status codes, retry flags, category markers). Trace error augmentation through every catch/re-throw to verify domain-specific metadata survives.

**Error diagnostic quality** — For each error return, ask: "Could I diagnose this from a production log without a debugger?" Flag: missing context (which file, which ID, what input), generic messages ("parse failed" without showing what), catching rich errors and re-throwing vague ones.

---

#### Documentation & project smells

**Documentation drift** — README, docstrings, or inline comments describing behavior differently from actual code. Config tables listing wrong defaults. File layout diagrams showing paths that don't exist. Comments saying "this validates X" when validation was removed. Architecture descriptions predating a refactor.

**Build reproducibility** — Language/runtime version not pinned or pinned to unreleased versions. Missing or uncommitted lockfiles. Setup instructions referencing files that don't exist. Build steps requiring undocumented system dependencies.

---

#### Risk indicators

**Complex + untested** — Method with deep nesting (3+ levels), long body (50+ lines), or high branching and no nearby test file. Also: catch/fallback blocks with no corresponding test exercising the error scenario. Test suites where error-path tests are below a 1:3 ratio to happy-path tests.

**High-churn area without tests** — Files with many TODO/FIXME comments, multiple revision markers, or (if git is available) 10+ commits in the last quarter with no corresponding test file.

**Tangled dependencies** — Constructor with 8+ parameters, circular references, or deeply coupled classes that would be hard to test in isolation.

---

### Logging format

When observing smells (embedded or standalone), log each finding as a one-liner in the source doc's `## Debt` section:

```markdown
## Debt

| Where                | Smell            | What                                                       | Effort |
| -------------------- | ---------------- | ---------------------------------------------------------- | ------ |
| `OrderService.php`   | SRP              | Handles both validation and fulfillment                    | Medium |
| `helpers.php`        | Dead code        | 3 unused functions: `formatLegacy`, `oldSlug`, `convertV1` | Low    |
| `routes/api.php`     | Misplaced        | Business logic in route closure                            | Low    |
| `UserController.php` | Raw SQL          | String concatenation in WHERE clause                       | High   |
| `ReportService.php`  | N+1              | Loading orders in loop instead of batch                    | Medium |
| `PaymentGateway.php` | Complex+untested | 80-line method, 6 nesting levels, no test file             | High   |
```

Keep entries terse. The debt doc created at `/close` time will expand them.

### Standalone mode

When invoked directly (`/sniff path/to/area`):

#### Evaluate skills

Follow `/skills`'s _Evaluate skills_ step.

#### Scan the target area

// turbo

Follow `/lib`'s _Research Deep_ level for the specified path. Apply every category in the _Smell checklist_ above.

#### Write findings

// turbo

Follow `/close`'s _Create debt doc_ step. Use the findings table as the `## Evidence` section and translate top items into `## Requirement` entries.

#### Present for review

Present the doc via `notify_user`. User decides which items to plan.
