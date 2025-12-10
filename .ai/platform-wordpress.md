# WordPress Coding Rules

## Important Guidelines

1. **Security First**: Always sanitize input and escape output
2. **WordPress Hooks**: Use WordPress hooks and filters appropriately
3. **Function Prefixes**: Prefix custom functions to avoid conflicts
4. **Nonce Verification**: Use nonces for form submissions
5. **Database Queries**: Use WordPress database functions, avoid raw SQL

## WordPress Integration

- Use WordPress functions when they provide needed functionality
- Always escape output: `esc_html()`, `esc_attr()`, `wp_kses()`
- Sanitize input: `sanitize_text_field()`, `sanitize_email()`
- Use prepared statements: `$wpdb->prepare()`

## Security Requirements

- Verify nonces for form submissions
- Check user capabilities before sensitive operations
- Use `wp_safe_redirect()` instead of `header('Location:')`
- Validate and sanitize all user input

## File Organization

- Plugin files should be in `/web/wp-content/plugins/wordpress-extras/`
- Follow WordPress plugin structure
- Use autoloading for classes
- Separate admin and public functionality

## Performance Guidelines

- Use WordPress caching functions when appropriate
- Minimize database queries
- Use WordPress transients for temporary data
- Optimize asset loading (CSS/JS)
