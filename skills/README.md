# Built-in Skills

This directory contains wukongbot-go's built-in skills. These skills extend the agent's capabilities for specific domains and tasks.

## Available Skills

| Skill | Description | Requirements |
|-------|-------------|--------------|
| [skill-creator](./skill-creator/) | Create and design new AgentSkills | None |
| [github](./github/) | Interact with GitHub using `gh` CLI | `gh` |
| [weather](./weather/) | Get weather forecasts (no API key) | `curl` |
| [summarize](./summarize/) | Summarize URLs, files, YouTube videos | `summarize` |
| [tmux](./tmux/) | Remote-control tmux sessions | `tmux`, macOS/Linux |
| [cron](./cron/) | Schedule reminders and recurring tasks | None |
| [swagger](./swagger/) | Query and interact with external APIs via Swagger/OpenAPI | None |

## Usage

Skills are automatically loaded when relevant to your request. For example:

- Ask about GitHub issues/PRs → `github` skill activates
- Ask about the weather → `weather` skill activates
- Need to create a new skill → `skill-creator` skill activates
- Need to query external APIs → `swagger` skill activates

## Adding Custom Skills

Create custom skills in your workspace:

```bash
~/wukongbot-workspace/skills/{skill-name}/SKILL.md
```

See the `skill-creator` skill for guidance on creating effective skills.

## Hooks Integration

The hooks system provides additional capabilities that can be configured in `config.yaml`:

- **backup-before-write**: Automatically backup files before writing
- **dangerous-command-check**: Block dangerous commands (rm -rf, format, etc.)
- **auto-test**: Automatically run tests after writing Go files
- **code-development-task**: Delegate coding tasks to external tools

These hooks are configured under `tools.hooks.code_development` in your config file and are automatically triggered based on the event type.

For detailed configuration, see the [Hooks section](../README.md#hooks-system) in the main README.
