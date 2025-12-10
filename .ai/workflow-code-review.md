# Code Review Workflow

1. **Context Load** – Reviewers must confirm the submitting agent loaded `.ai/index.md` and all dependencies.
2. **Checklist**
   - PSR-12 compliance verified (`./lando-phpcs`).
   - Security practices applied (nonces, escaping, sanitization).
   - Tests updated or added; `./lando-phpunit` passes locally.
   - Performance considerations addressed (caching, minimized queries).
3. **Approval Rules** – At least one language SME (PHP/JS) plus one platform SME (WordPress) must sign off before merge.
4. **Post-Review** – Merge author updates `.ai/meta/CHANGELOG.md` if policies changed, and communicates updates to extension maintainers.
