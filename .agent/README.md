# Agent Skills & Bundles

This directory contains AI agent skills and project-specific bundle configurations.

## Structure

```
.agent/
├── config/bundles/     # Project-specific skill bundles (source controlled)
├── skills/             # Skills library
├── workflows/          # Workflow definitions
└── rules/              # Agent rules
```

## Quick Start

### Generate a Project Bundle

Run the `/generate-bundle` workflow to analyze your codebase and create a custom bundle:

1. Ask your agent: "Use /generate-bundle to create a skills bundle for this project"
2. The agent will analyze the codebase, review available skills, and create a bundle at `.agent/config/bundles/<name>.md`

### Use a Bundle

Reference bundles in your prompts:

- "Use the wordpress bundle for this task"
- "Follow the skills in my project bundle"

## Skills Library

Skills are installed via Composer from [antigravity-awesome-skills](https://github.com/sickn33/antigravity-awesome-skills).

Update skills:

```bash
lando composer update sickn33/antigravity-awesome-skills
```

Browse available skills:

```bash
ls .agent/skills/
```

## Creating Custom Bundles

Create `.agent/config/bundles/<name>.md`:

```markdown
# My Bundle

## Description

What this bundle is for.

## Skills

- skill-name-1
- skill-name-2

## Usage Notes

Project-specific context.
```
