# Laravel Rules

> **Conditional**: Apply only when Laravel is detected (`artisan`, `composer.json` with `laravel/framework`). Skip for non-Laravel projects.

## Framework Config

- Never modify framework config file defaults — override via `.env`. Before modifying any value in a config file (e.g., `database.php`, `cache.php`), check git history for the original default. Config files define fallbacks; `.env` provides the actual values.
- Add new env vars to both `.env` and `.env.example` under the existing section header for the relevant feature.

## Facades

- Facade docblocks must mirror the underlying service's method signatures exactly — keep in sync when signatures change.

## Service Extraction

- When an external API client handles cross-cutting concerns (rate limiting, auth, retries), extract a dedicated client service that wraps the transport layer. Compose higher-level services via an abstract base that holds the shared client.

## Jobs & Queues

- Align all timeouts in the same chain: Horizon supervisor timeout ≥ job timeout ≥ HTTP client timeout. Mismatched values cause jobs to be killed mid-request.
- When a job needs a configurable timeout, define an env var and reference it via `config()`.

## Redis

- Isolate Redis databases by concern. Each concern gets its own DB number and its own cache store config.

## Artisan & Make

- When creating a new artisan command, also add a corresponding `make` target in the Makefile. Check existing targets for naming conventions.

## Testing

- When mocking a facade, use `shouldReceive()->andReturn()->byDefault()` rather than `::spy()` — spy returns null by default, which triggers typed property errors on value objects.
