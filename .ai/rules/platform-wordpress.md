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

## File Organization

- Follow WordPress plugin structure
- Use autoloading for classes
- Separate admin and public functionality

## Performance Guidelines

- Use WordPress caching functions when appropriate
- Minimize database queries
- Use WordPress transients for temporary data
- Optimize asset loading (CSS/JS)
