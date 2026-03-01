# PHP Rules

## Standards

- PSR-12 style, SOLID principles, `declare(strict_types=1)`.
- CRITICAL: Always start with the most restrictive visibility (`private` or `protected`). Only widen to `public` when external access is explicitly required.
- Type hints on all parameters/returns; `?` only for genuinely optional types.
- Constructor property promotion where applicable.
- Group `use` statements by type (classes, functions, constants).
- Early returns over deep nesting; `match` over long `switch`/`if` ladders.
- Pure functions, immutable value objects; `readonly` properties (8.2+); `BackedEnum` for closed sets.
- Domain-specific exceptions over returning `null`/`false`; never silence exceptions.
- Constructor injection over service locators; lean, explicit constructors.
- Strict comparisons (`===`); `DateTimeImmutable` + injected clocks; no `time()`/`date()` in domain.
- Type-safe collections via array shapes or static analysis annotations.
- PHPStan/Psalm at high level; ignores only with documented rationale.
- Prefer attributes over docblock annotations when available; PHPDoc only when types aren't obvious.
- No implicit output (`echo`/`var_dump`) in libraries; structured logging only.
- Small pure helpers over traits; if traits used, keep stateless.

## Organization

- PSR-4 namespaces; one class per file; `StudlyCaps` classes, `camelCase` methods/props, `UPPER_CASE` constants.
- Methods small, single-responsibility. Classes cohesive by bounded context — no god classes.
- Controllers handle I/O; services handle business rules; repositories handle storage; utilities stay framework-agnostic.
- Interfaces for cross-layer contracts. Co-locate tests with features when possible.
- Never park feature logic in UI classes.

## Namespace & Import Rules

- **Never use FQCN in code body** — always `use` at top and reference short name.
- **No redundant aliases** — `as` only for actual name collisions.
- Exceptions: inside `use` statements, unavoidable collisions, strings/comments.

## Constructor Parameters

- Required dependencies are required — no `?`/`= null` unless genuinely optional.
- Nullable acceptable only when class has explicit null-handling with meaningful fallback.

## Dependency Injection

- Always constructor injection; avoid `app()`, `Container::make()` mid-method.
- Service locators acceptable only in framework entry points (controllers, commands, handlers); document the reason.
