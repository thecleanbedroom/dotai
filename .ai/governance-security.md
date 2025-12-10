# Security & Privacy Governance

- Treat all generated code as if it will run in production; never include secrets, tokens, or customer data in prompts or outputs.
- Sanitize and validate every external input path; enforce capability checks (`current_user_can`) before sensitive operations.
- Prefer HTTPS endpoints, `wp_safe_redirect`, and prepared statements (`$wpdb->prepare`).
- Log security-relevant events (failed nonce checks, privilege escalations) using WordPress logging facilities.
- Escalate suspected vulnerabilities immediately via the security issue template in `.ai/templates/security-issue.md` (to be created when process is defined).
