---
name: github
description: Interact with GitHub using the `gh` CLI. Use for issues, PRs, CI runs, and advanced API queries.
metadata: {"requires":{"bins":["gh"]}}
---

# GitHub Skill

Use the `gh` CLI to interact with GitHub. Always specify `--repo owner/repo` when not in a git directory.

## Pull Requests

**Check CI status on a PR:**
```bash
gh pr checks 55 --repo owner/repo
```

**List recent workflow runs:**
```bash
gh run list --repo owner/repo --limit 10
```

**View a run and see which steps failed:**
```bash
gh run view <run-id> --repo owner/repo
```

**View logs for failed steps only:**
```bash
gh run view <run-id> --repo owner/repo --log-failed
```

## Issues

**List issues:**
```bash
gh issue list --repo owner/repo
```

**Create an issue:**
```bash
gh issue create --repo owner/repo --title "Bug title" --body "Description"
```

**Close an issue:**
```bash
gh issue close 123 --repo owner/repo
```

## API for Advanced Queries

The `gh api` command accesses data not available through other subcommands.

**Get PR with specific fields:**
```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```

**Search issues:**
```bash
gh api search/issues --jq '.[].number,.title' -q "repo:owner/repo is:issue"
```

## JSON Output

Most commands support `--json` for structured output with `--jq` to filter:

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
```

**Available JSON fields:**
- For issues/prs: `number`, `title`, `state`, `body`, `author`, `createdAt`
- For runs: `number`, `status`, `conclusion`, `createdAt`, `updatedAt`
