# WordPress Coding Rules

## Important Guidelines

1. **Security First**: Always sanitize input and escape output
2. **WordPress Hooks**: Use WordPress hooks and filters appropriately
3. **Function Prefixes**: Prefix custom functions to avoid conflicts
4. **Nonce Verification**: Use nonces for form submissions
5. **Database Queries**: Use WordPress database functions, avoid raw SQL

## WordPress Security

- Use `current_user_can()` (or capability-specific helpers) before privileged actions.
- Add and verify nonces (`wp_nonce_field()`, `check_admin_referer()`, `wp_verify_nonce()`) on every state-changing request, including REST and AJAX endpoints.
- Sanitize input with core helpers (`sanitize_text_field()`, `sanitize_email()`, `sanitize_key()`, etc.) as soon as data enters the system.
- Escape output at the last moment using `esc_html()`, `esc_attr()`, `esc_url()`, `wp_kses()`, and `wp_json_encode()` for structured responses.
- Use `$wpdb->prepare()` (or higher-level APIs like `WP_Query`) for database access; never concatenate untrusted strings into SQL.
- Prefer `wp_safe_redirect()` over raw header redirects to prevent open-redirect issues.
- Validate and sanitize all user input, including data flowing through hooks and CLI commands.
- For REST endpoints, require a strict `permission_callback`; never leave it `__return_true`. Validate and sanitize `WP_REST_Request` params before use.
- For AJAX, enforce capability + nonce checks in both authenticated and unauthenticated handlers; never rely on `wp_ajax_nopriv` alone.

## File Organization

- Follow WordPress plugin structure
- Use autoloading for classes
- Separate admin and public functionality
- Keep domain logic in services; keep admin screens/controllers thin and focused on I/O.
- Name functions/classes with a project-specific prefix/namespace; avoid globals.

## Performance Guidelines

- Use WordPress caching functions when appropriate
- Minimize database queries
- Use WordPress transients for temporary data
- Optimize asset loading (CSS/JS)

## Assets & Frontend

- Enqueue assets with `wp_enqueue_*` and dependency lists; never hardcode `<script>`/`<link>` tags. Version assets for cache busting (file mtime or build hash).
- Localize data via `wp_localize_script`/`wp_add_inline_script` only with sanitized/escaped values; prefer REST for dynamic data when possible.
- Register block assets via `block.json`; avoid embedding logic in `functions.php`.

## Internationalization

- Add translations only when requested; otherwise keep strings plain but ready for i18n. When adding i18n, wrap user-facing strings in translation functions (`__`, `_e`, `_x`, `esc_html__`, etc.) with the correct text domain.
- Avoid string concatenation for translatable text; use placeholders and `sprintf`/`wp_sprintf`.

## Data & Settings

- Register settings with `register_setting` and provide a `sanitize_callback`. Validate option arrays with explicit allowlists.
- Flush rewrite rules only on activation/deactivation, never on each request.

## Testing (WordPress)

- Use `WP_UnitTestCase`/integration harnesses for hooks, filters, and REST/AJAX routes.
- Reset globals, options, and hooks between tests; isolate filesystem/network calls unless under test.
- Rely on the site-level testing framework: plugins should not ship their own PHPUnit bootstrap, `phpunit.xml`, or test environment setup—reuse the central harness and configuration.
- Root level testing harness exists in /tests
- Coverage report exists at /tests/reports/coverage/index.html
