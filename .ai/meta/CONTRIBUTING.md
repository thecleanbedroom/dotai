# Policy Contribution Workflow

1. **Open an Issue** – Describe the policy gap, affected extensions, and risk level.
2. **Draft the Change** – Create or edit files under `.ai/`, keeping instructions product-agnostic. Include rationale and fallback instructions for agents lacking features.
3. **Update `index.md`** – Register every new file or directory path so agents discover it automatically.
4. **Run Validations** – Lint Markdown, verify JSON (e.g., `codestyle.json`), and load policies through at least one VS Code extension in a dry run.
5. **Request Review** – Tag SME reviewers (security, platform, language) for sign-off.
6. **Document in Changelog** – Summarize the change with date, author, and impact level.
7. **Communicate** – Notify plugin maintainers so they refresh cached policies.
