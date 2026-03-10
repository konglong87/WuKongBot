---
name: "code_dev"
description: "Code development skills for using external tools like opencode and Cursor"
always: false
---

# Code Development Skills

Use the **code_dev** tool to perform coding tasks using external AI-powered development tools like opencode and Cursor.

## When to Use

Use this skill when the user asks to:
- Code with external tools: "用opencode...", "编程：cursor..."
- Mentions "编程" (coding), "写代码" or related keywords
- Explicitly requests opencode or Cursor
- Needs to write, refactor, create, or modify code using these tools

## Available Tools

### opencode
- Full name: Claude Code Request
- Use when: User mentions "opencode" or "opencode"
- Best for: General coding tasks, code changes, new features

### Cursor
- Use when: User mentions "cursor" or "Cursor"
- Best for: AI-assisted development in Cursor editor

## Tool Parameters

```json
{
  "tool": "opencode|cursor",
  "task": "clear description of the coding task in Chinese"
}
```

**Parameters:**
- `tool` (required): The external tool to use - either "opencode" or "cursor"
- `task` (required): The coding task description in Chinese (since user communicates in Chinese)

## Examples

### Example 1: Using opencode
**User:** "用opencode添加登录功能"
**Call:** `code_dev(tool="opencode", task="添加登录功能")`

### Example 2: Using Cursor
**User:** "编程：cursor 创建支付接口"
**Call:** `code_dev(tool="cursor", task="创建支付接口")`

### Example 3: Bug fix
**User:** "opencode修复用户认证的bug"
**Call:** `code_dev(tool="opencode", task="修复用户认证的bug")`

### Example 4: Refactoring
**User:** "用cursor重构订单处理代码"
**Call:** `code_dev(tool="cursor", task="重构订单处理代码")`

### Example 5: Developing with opencode
**User:** "用cc实现用户登录模块代码"
**Call:** `code_dev(tool="opencode", task="实现用户登录模块代码")`

## How to Extract Task Description

When user says "用opencode写一个Hello Wakkk":
- Identify tool: "opencode"
- Extract task: "写一个Hello Wakkk"

When user says "编程：cursor添加支付接口":
- Identify tool: "cursor"
- Extract task: "添加支付接口"

When user says "opencode \"Write a Hello Wakkk\"":
- The user is giving the exact command format
- Tool: "opencode"
- Task: "Write a Hello Wakkk" (keep in English if that's what user intends)

## Progress Monitoring

The system will automatically log:
- Task start (when tool execution begins)
- Task completion (when tool returns result)
- Any errors or timeouts

You don't need to manually track progress - the hooks system handles it.

## Tool Availability

Before calling, the system checks if the requested tool is installed:
- If `opencode` is not available, the tool will return an error
- If `cursor` is not available, the tool will return an error

Report any availability issues to the user in a friendly way.

## Tips

1. **Use Chinese for task descriptions**: Since users communicate in Chinese, extract the task in Chinese form
2. **Keep task descriptions concise**: External tools work best with clear, focused task descriptions
3. **Handle tool unavailability gracefully**: If a tool isn't available, let the user know and suggest alternatives
4. **Preserve exact quotes**: If user provides quoted text (like "Write a Hello Wakkk"), preserve it exactly

## Common Patterns

| User Input | Tool | Task |
|-----------|------|------|
| "用opencode写一个用户登录功能" | opencode | 写一个用户登录功能 |
| "编程：cursor创建API" | cursor | 创建API |
| "opencode修复这个bug" | opencode | 修复这个bug |
| "cursor重构代码" | cursor | 重构代码 |
| "opencode \"Create login\"" | opencode | Create login |
| "编程：添加支付功能" | Use default tool or ask | 添加支付功能 |

## Notes

- Do NOT handle "编程" without a specific tool - ask user which tool they prefer
- The system has a timeout configured (default 5 minutes)
- External tools run asynchronously and return their full output
- opencode is equal to cc and equal to claude code
