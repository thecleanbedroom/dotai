# Laravel Modules (nwidart/laravel-modules v12)

> **Conditional**: Apply only when `nwidart/laravel-modules` is detected in `composer.json`. Skip for non-modular Laravel projects.

## Package

- Uses `nwidart/laravel-modules` v12+ — [docs](https://laravelmodules.com/docs/12).
- Module activation tracked in `modules_statuses.json` (root). Set to `true` to enable, `false` to disable.
- Module metadata lives in `module.json` (per module). The `priority` key controls boot/seed order (0 = default).

## Directory Structure

All modules live under `Modules/` at the Laravel root. Each module mirrors a miniature Laravel app — same `app/`, `config/`, `database/`, `routes/`, `resources/`, and `Tests/` directories you'd expect, plus `module.json` (metadata/providers) and a `composer.json` (informational deps).

## Namespace

- Root namespace: `Modules\{ModuleName}\App\`
- Example: `Modules\RetailRocket\App\Services\WooRestService`
- The `app/` folder is the application root — not `src/`. This is configured via the `app_folder` setting in `config/modules.php`.

## Service Provider

- Every module has exactly one main service provider, registered in `module.json` under `providers`.
- The service provider is the **only entry point** — all bindings, config registration, route loading, and event listeners go here.
- Use `config()->set()` in the service provider to register dynamic config (Redis connections, cache stores, Horizon supervisors) — never modify root Laravel config files.

## File Isolation

- Keep module files inside the module directory. Check for existing module-level directories (`config/`, `routes/`, `resources/`, etc.) before creating files in root Laravel directories.
- If a module file truly can't live inside the module, notify the user and ask before placing it elsewhere.

## Facades

- When a module service is called from 3+ sites, expose it via a facade. Register the binding in the module's service provider, create the facade in `app/Facades/`.

## Jobs & Queues

- Each module owns its own Horizon supervisor config — register in the module's service provider, not in root `horizon.php`.

## Redis

- Each concern gets its own cache store config registered via `config()->set()` in the service provider.

## Testing

- Module tests are scoped to `Modules/{ModuleName}/Tests/`. Run with `TESTDIR` or `--filter` to avoid running unrelated modules.

## Config

- Module config files live in `config/` within the module directory.
- Access via `config('modulename.key')` — e.g., `config('retailrocket.sync_interval')`.
- Multiple config files are supported (v11.1.5+): `config('modulename.filename.key')`.
- Migrations in `database/migrations/` are auto-discovered — no manual registration needed.

## Artisan Commands

Prefer `module:make-*` generators over manual file creation:

| Command                                  | Creates                    |
| ---------------------------------------- | -------------------------- |
| `module:make {Name}`                     | Entire new module scaffold |
| `module:make-model {Name} {Module}`      | Model in module            |
| `module:make-controller {Name} {Module}` | Controller in module       |
| `module:make-command {Name} {Module}`    | Console command in module  |
| `module:make-job {Name} {Module}`        | Job in module              |
| `module:make-service {Name} {Module}`    | Service in module          |
| `module:make-test {Name} {Module}`       | Test in module             |
| `module:make-migration {Name} {Module}`  | Migration in module        |
| `module:make-request {Name} {Module}`    | Form request in module     |
| `module:make-middleware {Name} {Module}` | Middleware in module       |
| `module:make-factory {Name} {Module}`    | Factory in module          |
| `module:make-interface {Name} {Module}`  | Interface in module        |

Utility commands:

- `module:list` — list all modules and their status
- `module:enable {Name}` / `module:disable {Name}` — toggle activation
- `module:migrate {Name}` — run module migrations
- `module:seed {Name}` — run module seeders

## Module Facade

Access module instances programmatically:

```php
use Nwidart\Modules\Facades\Module;

Module::all();                    // All modules
Module::find('RetailRocket');     // Specific module
Module::allEnabled();             // Only enabled
Module::isEnabled('ModuleName');  // Check status
Module::getPath();                // Modules root path
```

## Helpers

- `module_path('ModuleName')` — returns absolute path to module root
- `module_path('ModuleName', 'app/Services/Foo.php')` — returns path to specific file

## Cross-Module Dependencies

- Modules should minimize coupling. If Module B depends on Module A, use interfaces and service container bindings rather than direct class references.
- Set the `priority` key in `module.json` when boot order matters (lower = boots first).
- Module-specific `composer.json` dependencies are informational only — actual packages must be in the root `composer.json`.
