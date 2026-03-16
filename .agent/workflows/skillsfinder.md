---
description: Generate a project-specific skills bundle by analyzing the codebase
---

# Generate Skills Bundle

Clone the skills repo, analyze the project and workflows, reason about which skills match, and install them. Every run performs a full evaluation covering both universal skills (platform-agnostic, workflow-complementing) and project-specific skills.

// turbo-all

## Steps

### Clone skills repo to tmp

```bash
SKILLS_TMP=$(mktemp -d)
echo "$SKILLS_TMP" > /tmp/.skills_tmp_path
echo "Cloning skills to $SKILLS_TMP..."
git clone --depth 1 https://github.com/sickn33/antigravity-awesome-skills.git "$SKILLS_TMP" 2>&1
if [ $? -ne 0 ]; then
  echo "ERROR: Clone failed"
  exit 1
fi
echo "Done. Skills available:"
ls "$SKILLS_TMP/skills/" | wc -l
```

If the clone fails, stop and tell the user.

### Extract frontmatter catalog

One skill per line: `slug - description (NKB)`. Size includes all files in the skill folder.

```bash
SKILLS_TMP=$(cat /tmp/.skills_tmp_path)
python3 -c "
import os, re, subprocess

base = os.path.join('$SKILLS_TMP', 'skills')
lines = []
for name in sorted(os.listdir(base)):
    skill_dir = os.path.join(base, name)
    skill_md = os.path.join(skill_dir, 'SKILL.md')
    if not os.path.isfile(skill_md):
        continue
    total_size = int(subprocess.check_output(['du', '-sb', skill_dir]).split()[0])
    desc = ''
    with open(skill_md) as f:
        content = f.read(2000)
    fm = re.match(r'^---\s*\n(.*?)\n---', content, re.DOTALL)
    if fm:
        for line in fm.group(1).splitlines():
            if line.strip().lower().startswith('description:'):
                desc = line.split(':', 1)[1].strip().strip('\"').strip(\"'\")[:150]
                break
    if not desc:
        body = content.split('---', 2)[-1].strip() if '---' in content else content
        for line in body.splitlines():
            line = line.strip()
            if line and not line.startswith('#') and not line.startswith('---'):
                desc = line[:150]
                break
    size_kb = f'{total_size/1024:.1f}KB'
    lines.append(f'{name} - {desc} ({size_kb})')

with open('/tmp/.skills_catalog.txt', 'w') as f:
    f.write('\n'.join(lines) + '\n')
print(f'Catalog: {len(lines)} skills')
"
```

### Analyze project and workflows

**Project profile** — explore the codebase freely (file extensions, directory structure, dependency files, infrastructure, CI configs) until you understand:

- What languages/frameworks are used
- What the project actually does (its domain)
- How it's built, tested, and deployed
- What kinds of problems developers face working on it

Write a 3-5 line project profile summarizing the tech stack, architecture, domain, and key concerns.

**Workflow profile** — read every file in `.agent/workflows/`. For each, extract:

- **Purpose**: what it does (from description and intro)
- **Key steps**: the major actions the agent performs
- **Skill gaps**: what kinds of knowledge would make the agent better at executing each step

Build a workflow profile summarizing what the agent is asked to do across all workflows.

### Read catalog and reason about skill selection

Read `/tmp/.skills_catalog.txt` using `view_file`. You now have:

- The **project profile** from _Analyze project and workflows_ (what this project is)
- The **workflow profile** from _Analyze project and workflows_ (what the agent is asked to do)
- The **full skill catalog** with slug + description for every skill

Do NOT use grep, keyword matching, or string filtering. Read the catalog and reason about fit.

For each candidate, read its full `SKILL.md` to verify it delivers actionable guidance (not just a thin description wrapper).

#### Category A: Universal skills

Skills that every project needs regardless of platform. Select in two sub-groups:

**A1 — Universal production skills**: foundational software engineering concerns (clean code, debugging, testing methodology, code review). Must be genuinely platform-agnostic.

**A2 — Workflow complement skills**: skills that make installed workflows more effective. Each must cite a specific workflow and step it strengthens.

**Universal selection rules:**

- Must be genuinely **platform-agnostic** — skip language/framework-specific skills
- Pick the most **comprehensive** skill per concern
- When multiple skills cover the same concern, compare depth: prefer actionable guidance (checklists, examples, processes) over theory-only
- Prefer `risk: safe` over `risk: unknown` when quality is comparable
- Max ONE skill per concern area

#### Category B: Project-specific skills

Skills relevant to THIS project's tech stack, domain, and challenges.

**Project-specific selection rules:**

- Select based on the project profile, not just file extensions
- Skip skills for languages/frameworks/platforms NOT in the project
- Skip penetration testing unless it's a security project
- Prefer specific skills (e.g., `laravel-expert`) over generic ones (e.g., `backend-architect`) when both exist

> ❌ Bad: "project has PHP files → pick `php-pro`"
> ✅ Good: "project has complex OOP patterns, generators, SPL usage → `php-pro` helps write idiomatic PHP"

**Combined budget**: 10-20 skills total across both categories. Max ONE skill per concern area.

### Install selected skills

```bash
SKILLS_TMP=$(cat /tmp/.skills_tmp_path)

# Remove existing skills
find .agent/skills/ -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} +
find .agent/skills/ -mindepth 1 -maxdepth 1 -type l -exec rm -f {} +
mkdir -p .agent/skills

# Copy each selected skill
for skill in <space-separated slugs from skill selection>; do
  if [ -d "$SKILLS_TMP/skills/$skill" ]; then
    cp -r "$SKILLS_TMP/skills/$skill" ".agent/skills/$skill"
    echo "✅ $skill"
  else
    echo "⚠️  $skill not found in repo"
  fi
done
```

### Clean up

```bash
SKILLS_TMP=$(cat /tmp/.skills_tmp_path)
rm -rf "$SKILLS_TMP"
rm -f /tmp/.skills_tmp_path /tmp/.skills_catalog.txt
echo "Cleaned up."
```

### Verify

```bash
echo "Skills installed:"
count=$(ls -1d .agent/skills/*/ 2>/dev/null | wc -l)
echo "Count: $count"
echo ""
for d in .agent/skills/*/; do
  name=$(basename "$d")
  if [ -f "$d/SKILL.md" ]; then
    echo "✅ $name"
  else
    echo "❌ $name (missing SKILL.md)"
  fi
done
```

Confirm count is ≤ 20 and every directory has a SKILL.md.

## Output

Report to the user:

### Project Profile

[3-5 line summary from _Analyze project and workflows_]

### Selected Skills (N/20)

#### Universal Skills

| #   | Skill | Category   | Concern                  | Supports       |
| --- | ----- | ---------- | ------------------------ | -------------- |
| 1   | slug  | Production | e.g., Clean code / SOLID | All projects   |
| 2   | slug  | Workflow   | e.g., Planning           | `/plan` step X |

#### Project-Specific Skills

| #   | Skill      | Reason                             |
| --- | ---------- | ---------------------------------- |
| 1   | skill-name | Why it matches a real project need |

### Notable Exclusions

| Skill      | Reason                                |
| ---------- | ------------------------------------- |
| skill-name | Why excluded despite seeming relevant |

### Load installed skills

After reporting, follow /lib:#Evaluate Skills# to read and evaluate the newly installed skills for the current task.
