# Workflow Authoring Rules

Rules for writing and modifying `.agent/workflows/` files.

## Conventions

- **No step numbers in headings**: Use descriptive names (`### Run tests`), not numbered names (`### 3. Run tests`). Document order defines execution order. Numbers create maintenance overhead when steps are inserted or reordered.
- **Named step references, not numbers**: Cross-reference steps using the colon-hash syntax: `/workflow:#Section Title#`. Never reference by step number.
- **Cross-references must resolve**: Every named reference (e.g., /close:#Move to Finished#) must point to an actual heading in the target workflow. Dangling references cause agent confusion.
- **DRY — reference, don't redefine**: If a step already exists in another workflow, reference it instead of duplicating the instructions. One-liner references like "Follow /skillsfinder:#Clone Skills Repo to Tmp#" are preferred over inline code blocks that repeat the same logic.
- **Canonical ownership**: Each shared concern has one canonical workflow that owns its definition. Other workflows reference it. If you need to override behavior, say "Follow X with these overrides:" and list only the differences. Known owners:
  - Append Walkthrough, Finalize, Move to Finished, Create Debt Doc, Report → `/close`
  - Clone Skills Repo, Extract Catalog → `/skillsfinder`
  - Smell Checklist, Logging Format → `/sniff`
  - Evaluate Skills, Canonical Document Format, Resolve Input, Research, Tracing, Classification, QA Verification, Risk Analysis, Persona Definitions, Create Debt Document → `/lib`
  - Commit Message Format, Git Trailers → `/commit`
- **Platform-agnostic language**: Workflows must not contain hardcoded commands, language-specific patterns, or tool-specific names. Use generic terms ("run the test suite", "run static analysis") and let `.agent/rules/platform-*.md` or `language-*.md` supply the specifics.
- **Unique step names**: Every step heading within a workflow must be unique. Duplicate names make references ambiguous.
- **Globally unique referenced headings**: If a step heading is cross-referenced by other workflows, it must be unique across all workflow files. Two workflows defining the same heading name creates ambiguous references. Generic headings never used as cross-reference targets (e.g., "Summary", "Report") are exempt.
- **YAML frontmatter required**: Every workflow must start with YAML frontmatter containing at least a `description` field. The description should clearly explain the workflow's purpose and when to use it.
- **Turbo annotations**: Mark steps for auto-execution with `// turbo` (single step) or `// turbo-all` (entire workflow). Rules:
  - Read-only and idempotent steps (reading files, listing dirs, running tests, cloning repos) are safe to turbo.
  - Interactive and approval steps (presenting for review, asking user input, iterating on feedback) must never be turboed.
  - `// turbo-all` makes individual `// turbo` annotations redundant — don't use both.
  - `// turbo-all` is appropriate only when every step in the workflow is safe to auto-execute.
