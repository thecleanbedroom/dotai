# Php Coding Rules

## PHP Development Rules

## General PHP Standards

- Follow PSR-12 coding style
- Follow SOLID principles
- Always use strict types: `declare(strict_types=1);`
- Use type hints for all method parameters and return types
- Use `?` for nullable types
- Use constructor property promotion where applicable
- Group use statements by type (classes, functions, constants)
- Instead of deep nesting conditionals, return early
- Prefer pure functions and immutable value objects; avoid shared mutable state.
- Default to `readonly` properties (PHP 8.2+) and `enum`/`BackedEnum` for closed sets instead of string constants.
- Use `match` over long `switch`/`if` ladders; keep guard clauses for early exits.
- Throw domain-specific exceptions instead of returning `null`/`false`; never silence exceptions.
- Favor dependency injection over service locators/singletons; keep constructors lean and explicit.
- Require strict scalar comparisons (`===`, `!==`) and avoid loose truthiness checks.
- Normalize time handling with `DateTimeImmutable` and injected clocks; avoid `time()`/`date()` in domain logic.
- Keep collections type-safe via array shape docs or static analysis annotations; avoid mixed arrays of dissimilar types.
- Enforce static analysis (PHPStan/Psalm) at high level; allow ignores only with documented rationale.
- Prefer native attributes over docblock annotations when available (e.g., ORM/serializer metadata).
- Document public APIs with concise PHPDoc only when types aren’t obvious; avoid redundant comments.
- Ban implicit output in libraries (no `echo`/`var_dump`); use structured logging instead.
- Prefer small, pure helpers over traits; if traits are used, keep them stateless.

## Code Organization

- Use namespaces following PSR-4
- One class per file
- Class names in `StudlyCaps`
- Method names in `camelCase`
- Property names in `camelCase`
- Constants in `UPPER_CASE`
- Add PHPDoc only when the intent, types, or edge cases are not obvious from signatures and names; avoid redundant docblocks on self-explanatory code.
- Keep methods small and focused (single responsibility)
- Use return type declarations
- Keep classes cohesive: group behaviors by bounded context; avoid “god” classes that mix admin UI, domain logic, and persistence.
- Place code where it belongs: controllers/admin screens handle I/O and orchestration; domain/services handle business rules; repositories/persistence adapters handle storage; utilities stay framework-agnostic.
- Prefer interfaces for cross-layer contracts; keep implementations in feature-specific namespaces (e.g., `Queue\Admin`, `Queue\Domain`, `Queue\Infrastructure`).
- Do not park feature logic in UI classes (e.g., queue operations stay in queue services, not `AdminInterface`); move misplaced methods before adding new ones.
- Co-locate tests and fixtures with their feature/module when possible for clarity.

## PHP Namespacing Rule

Never use fully-qualified namespaces (FQCN) in the middle of PHP code. Always use `use` statements at the top of the file and reference the short class name or an alias.

### Bad

```php
$logger = rr()->make(\Illuminate\Log\Logger::class);
```

### Good

```php
use Illuminate\Log\Logger;
...
$logger = rr()->make(Logger::class);
```

### Exceptions

- Inside `use` statements themselves.
- When there is a name collision that cannot be resolved with an alias (rare).
- Inside strings and comments (though preferred to use the short name even there if possible).

## PHP Import & Aliasing Policy

To maintain codebase searchability and consistency, follow these rules for PHP `use` statements:

### 1. No Redundant Aliasing

Never use [as](cci:1://file:///home/eric/websites/codecide/backbay/local/web/wp-content/plugins/wordpress-extras/plugins-woo/retailrocket/retailrocket/src/WooConnection.php:251:4-290:5) aliases (e.g., `use Namespace\Class as Alias;`) unless there is a direct name collision within the same file. Always prefer the original short class name.

- **Bad**: `use Illuminate\Database\Capsule\Manager as DbCapsule;`
- **Good**: `use Illuminate\Database\Capsule\Manager;`

### 2. Mandatory Local Imports

Avoid fully-qualified class names (FQCN) in the body of the code (e.g., `\Exception` or `\RetailRocket\Http\Client`). Always import the class at the top of the file.

- **Bad**: `$client = new \RetailRocket\Http\Client();`
- **Good**:
  ```php
  use RetailRocket\Http\Client;
  ...
  $client = new Client();
  ```

### 3. Exception: Name Collisions

Aliases are **only** permitted when two imported classes share the same short name, or when an imported class shares the same name as the class being defined.

use GuzzleHttp\Client as GuzzleClient;
use RetailRocket\Http\Client; // Internal client takes precedence or is aliased if needed

````

## Constructor Parameter Rules

### Strict Null Handling

- **Never allow nullable constructor parameters** unless the parameter is genuinely optional.
- If a class dependency is required for the object to function, make it required (no `?` or `= null`).
- Nullable parameters signal "this is optional" - use them only when the class can legitimately operate without the dependency.

### Bad

```php
public function __construct(?Logger $logger = null) // Wrong: if logging is required, require it
````

### Good

```php
public function __construct(Logger $logger) // Correct: required dependency is required
```

### Exception

Nullable is acceptable when:

- The dependency enables optional functionality (e.g., optional caching layer)
- The class has explicit null-handling logic that provides a meaningful fallback

## Dependency Injection Policy

### Prefer Constructor Injection

- **Always use dependency injection** - pass dependencies through constructors, not via service locators or factories mid-method.
- Avoid calling `app()`, `Container::make()`, or similar service locators in the middle of methods. This is a code smell.
- If a dependency is needed, inject it through the constructor or method signature.

### Bad

```php
public function processOrder(): void
{
    $validator = app(OrderValidator::class); // Code smell: hidden dependency
    $mailer = app()->make(Mailer::class);    // Code smell: untraceable dependency
}
```

### Good

```php
public function __construct(
    private OrderValidator $validator,
    private Mailer $mailer
) {}

public function processOrder(): void
{
    $this->validator->validate($order);
    $this->mailer->send($confirmation);
}
```

### Exception

Service locator patterns may be acceptable when:

- In framework bootstrapping or entry points (controllers, commands, event handlers)
- Injecting the dependency would create excessive complexity (rare)
- Document the reason if using `app()` outside constructors
