# Testing & Tooling Expectations

- **Static Analysis**: Run `./lando-phpcs` (or PHPCS locally) before submitting changes; use the provided `phpcs.xml` profile.
- **Auto-fixes**: Apply `./lando-phpcbf` for PSR-12 formatting issues where safe.
- **Unit Tests**: Execute `./lando-phpunit` covering root `/tests` and plugin-level suites.
- **Front-end Builds**: Follow WordPress JS/CSS standards; run npm/yarn scripts as defined per project.
- **Reporting**: Surface tool output in pull requests so reviewers see pass/fail status without rerunning commands.
