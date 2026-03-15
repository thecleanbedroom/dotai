---
description: "Canonical code smell checklist — used standalone for targeted scans or referenced by /plan, /implement, /sweep-eval for continuous observation"
---

# /sniff — Code Smell Detection

Canonical checklist for identifying code smells, structural issues, and quality concerns. Used two ways:

**Standalone** — `/sniff path/to/area` — targeted scan of a specific area, outputs findings to a debt doc
**Embedded** — referenced by `/plan`, `/implement`, `/sweep-eval`, and `/testcoverage` for continuous observation while working

> [!IMPORTANT]
> Sniff is **eyes-only observation**. You notice things while reading code — you don't run commands, tools, or analysis. For active scanning (running tests, coverage, linters), use `/sweep-eval`.

### Smell checklist

Apply these checks to every file you read or touch. In embedded mode, observe passively and log findings. In standalone mode, scan systematically.

---

#### Structural smells

**SRP violations** — God classes, files doing too many things, methods with 5+ responsibilities. Look for classes with 10+ public methods or files over 500 lines that aren't data/config.

**DRY violations** — Duplicated logic across files, copy-pasted code with minor variations, parallel hierarchies that could be unified. Near-identical methods in sibling classes.

**Naming** — Files, classes, or methods that don't match what they do. Inconsistent naming conventions within the same area (mixed camelCase/snake_case). Names that would make a newcomer guess wrong.

**Misplaced code** — Business logic in controllers/views/routes, framework code in domain layer, config in the wrong directory, utilities that belong in a shared location. Flat directories with too many files that should be grouped by concern.

**Dead code** — Unused functions, unreachable branches, stale imports, commented-out code blocks serving no purpose, orphaned test files for deleted classes.

**Organization** — Files in wrong directories, missing test files for production code, inconsistent directory structure across similar modules, deeply nested hierarchies that could be flattened.

**Interface violations** — Public methods that should be private, missing type hints, overly permissive visibility, nullable parameters that aren't genuinely optional, leaky abstractions.

**Magic values** — Hardcoded strings, numbers, or URLs that should be constants or config values. Repeated literal values across files.

**Unclear boundaries** — Overlapping responsibilities between classes or modules. When it's not obvious which one owns a concern.

**Friction points** — Non-obvious conventions, missing abstractions, code you have to read 5 files to understand, undocumented implicit dependencies.

**Standards gaps** — Deviations from the project's own rules (`.agent/rules/`), framework best practices, or widely-accepted principles (SOLID, clean architecture, etc.).

---

#### Security smells

**Hardcoded secrets** — API keys, passwords, tokens, or credentials in source code instead of environment variables.

**Unvalidated input** — User input used directly without sanitization or validation at boundaries. Missing type checks on external data.

**Unsafe output** — HTML/attributes/URLs rendered without escaping. User content echoed directly.

**Raw SQL** — String concatenation in queries instead of prepared statements or parameterized queries.

**Missing auth checks** — Privileged actions without capability/role verification. Missing anti-forgery tokens on state-changing requests.

---

#### Performance smells

**N+1 queries** — Query inside a loop, or loading related records one at a time instead of batch/eager loading.

**Blocking calls in request path** — External HTTP calls, heavy computation, or file I/O in the main request without timeout/async.

**Unbounded results** — Queries or collections without pagination or limits. `findAll()` on potentially huge tables.

**Repeated expensive work** — Same computation or query executed multiple times without caching. Cache keys that aren't scoped or invalidated correctly.

**Missing timeouts** — External calls without timeout configuration. Queue jobs without timeout alignment.

---

#### Risk indicators

**Complex + untested** — Method with deep nesting (3+ levels), long body (50+ lines), or high branching and no nearby test file. Note: sniff can't verify this with tooling — just observe if it _looks_ risky.

**High-churn area without tests** — Frequently modified files (you can tell from the import/require patterns, TODO comments, or change history) with no corresponding test coverage.

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
