---
name: skill-creator
description: Create or update AgentSkills. Use when designing, structuring, or packaging skills with scripts, references, and assets.
---

# Skill Creator

This skill provides guidance for creating effective skills.

## About Skills

Skills are modular, self-contained packages that extend the agent's capabilities by providing specialized knowledge, workflows, and tools. Think of them as "onboarding guides" for specific domains or tasks.

## Core Principles

### Concise is Key

The context window is a public good. Skills share the context window with everything else the agent needs: system prompt, conversation history, other Skills' metadata, and the actual user request.

**Default assumption: the agent is already very smart.** Only add context the agent doesn't already have.

Prefer concise examples over verbose explanations.

### Set Appropriate Degrees of Freedom

Match the level of specificity to the task's fragility and variability:

**High freedom (text-based instructions)**: Use when multiple approaches are valid.

**Medium freedom (pseudocode or scripts with parameters)**: Use when a preferred pattern exists.

**Low freedom (specific scripts)**: Use when operations are fragile and consistency is critical.

### Anatomy of a Skill

Every skill consists of a required SKILL.md file and optional bundled resources:

```
skill-name/
├── SKILL.md (required)
│   ├── YAML frontmatter metadata (required)
│   └── Markdown instructions (required)
└── Bundled Resources (optional)
    ├── scripts/          - Executable code
    ├── references/       - Documentation
    └── assets/           - Files used in output
```

#### SKILL.md Format

Every SKILL.md consists of:

- **Frontmatter** (YAML): Contains `name` and `description` fields. Be clear and comprehensive in describing what the skill does and when it should be used.
- **Body** (Markdown): Instructions and guidance for using the skill. Only loaded AFTER the skill triggers.

#### When to Include Bundled Resources

**Scripts (`scripts/`)**: Include when the same code is being rewritten repeatedly or deterministic reliability is needed.

**References (`references/`)**: Documentation the agent should reference while working. Examples: API docs, schemas, company policies.

**Assets (`assets/`)**: Files used in the final output (templates, images, boilerplate code).

## Skill Creation Process

1. **Understand the skill** with concrete examples
2. **Plan reusable skill contents** (scripts, references, assets)
3. **Create the skill directory** with SKILL.md
4. **Implement resources** (scripts, references, assets)
5. **Test and iterate**

### Step 1: Understanding with Examples

Ask concrete questions to understand the skill's purpose:

- "What functionality should this skill support?"
- "Can you give examples of how this skill would be used?"
- "What would a user say that should trigger this skill?"

### Step 2: Planning Contents

Analyze each example to identify reusable resources:

- Scripts for deterministic operations
- References for documentation and schemas
- Assets for templates and boilerplate

### Step 3: Create Skill Directory

Create the skill directory structure:

```bash
mkdir -p ~/wukongbot-workspace/skills/{skill-name}
mkdir -p ~/wukongbot-workspace/skills/{skill-name}/scripts
mkdir -p ~/wukongbot-workspace/skills/{skill-name}/references
mkdir -p ~/wukongbot-workspace/skills/{skill-name}/assets
```

### Step 4: Write SKILL.md

**Naming**: Use lowercase letters, digits, and hyphens only.

**Frontmatter format**:
```yaml
---
name: "skill-name"
description: "What the skill does. Include when to use it."
---
```

**Body**: Write concise instructions. Use examples. Avoid verbose explanations.

### Step 5: Testing

Test the skill on real tasks. Iterate based on feedback.

## Progressive Disclosure

Skills use a three-level loading system:

1. **Metadata (name + description)** - Always in context (~100 words)
2. **SKILL.md body** - When skill triggers (<5k words)
3. **Bundled resources** - As needed

Keep SKILL.md under 500 lines. Move detailed content to reference files.

## What to Avoid

Do NOT create auxiliary files:
- README.md
- INSTALLATION_GUIDE.md
- QUICK_REFERENCE.md
- CHANGELOG.md

Keep only what's essential for the agent to do the job.
