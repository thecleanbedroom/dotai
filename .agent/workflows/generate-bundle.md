---
description: Generate a project-specific skills bundle by analyzing the codebase
---

# Generate Skills Bundle

This workflow analyzes your codebase and available skills to create a project-specific bundle.

## Steps

1. **Analyze the codebase** to identify:
   - Primary languages (PHP, JavaScript, CSS, etc.)
   - Frameworks (WordPress, Laravel, React, etc.)
   - Key patterns (testing, security, API, etc.)
   - Project type (plugin, theme, application, etc.)

2. **List available skills** from `.agent/skills/`:

   ```bash
   ls .agent/skills/
   ```

3. **Match skills to project needs** by reading SKILL.md files in relevant skill directories to understand their purpose.

4. **Create bundle file** at `.agent/rules/project-bundle.md` with format:

   ```markdown
   # <Bundle Name>

   ## Description

   Brief description of what this bundle is for.

   ## Skills

   - skill-name-1
   - skill-name-2
   - skill-name-3

   ## Usage Notes

   When to use this bundle and any project-specific context.
   ```

5. **Verify** the bundle references valid skills that exist in `.agent/skills/`.

## Example Skills to Consider

### For WordPress Projects

- `wordpress-plugin-development`
- `php-patterns`
- `security-audit`
- `testing-patterns`

### For JavaScript Projects

- `typescript-expert`
- `react-patterns`
- `testing-patterns`

### For DevOps

- `docker-expert`
- `aws-serverless`
- `workflow-automation`

## Output

The workflow produces a bundle file that agents can reference to know which skills are relevant for this project.
