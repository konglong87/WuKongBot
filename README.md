# wukongbot-go

[![CI](https://github.com/konglong87/wukongbot/actions/workflows/ci.yml/badge.svg)](https://github.com/konglong87/wukongbot/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/konglong87/wukongbot)](https://goreportcard.com/report/github.com/konglong87/wukongbot)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A lightweight AI assistant framework written in Go, ported from the Python version. Supports multiple channels (Telegram, WhatsApp, Feishu) and multiple LLM providers.

> **中文文档**: [README_CN.md](README_CN.md) | **English Documentation**: [README.md](README.md)

## Features

- **Multi-channel Support**: Telegram, WhatsApp, Feishu/Lark
- **Multi-LLM Support**: OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini, Groq, Zhipu, vLLM, and more
- **Tool System**: File operations, shell commands, web search/fetch, subagents, cron jobs, image generation/analysis
- **Hooks System**: Event-driven hooks for custom behaviors at lifecycle events (PreToolUse, PostToolUse, code development)
- **Claude Code Integration**: Real-time interactive coding sessions via Feishu with PTY-based Claude Code CLI integration
- **Tmux Interactive Cards**: 🆕 Dangerous command confirmation, interactive question detection, Feishu card interaction, session management
- **Persistent Memory**: SQLite or MySQL storage + file-based memory
- **Skills System**: Markdown-based skill definitions with YAML frontmatter, progressive loading
- **Subagents**: Background task execution with result announcements
- **Cron Scheduling**: Schedule messages and recurring tasks
- **Swagger/OpenAPI Integration**: Auto-generate API tools from OpenAPI specifications
- **Image Capabilities**: Text-to-image and image-to-text using Qwen's WanX and Qwen-VL models
- **Agent Team**: Multi-agent collaboration with automatic task decomposition and intelligent distribution

## Installation

### From Source

```bash
git clone https://github.com/konglong87/wukongbot.git
cd wukongbot
go build -o wukongbot ./cmd/wukongbot
```

### Build for Different Platforms

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o wukongbot-linux-amd64 ./cmd/wukongbot

# macOS
GOOS=darwin GOARCH=arm64 go build -o wukongbot-arm64 ./cmd/wukongbot
GOOS=darwin GOARCH=amd64 go build -o wukongbot-amd64 ./cmd/wukongbot

# Windows
GOOS=windows GOARCH=amd64 go build -o wukongbot.exe ./cmd/wukongbot
```

## Quick Start

### 1. Create Configuration File

Create `~/.wukongbot/config.yaml`:

```yaml
agents:
  defaults:
    workspace: "~/wukongbot-workspace"
    model: "gpt-4o"
    max_tokens: 4096
    temperature: 0.7
    max_tool_iterations: 10

providers:
  openai:
    api_key: "sk-xxx"

channels:
  telegram:
    enabled: false
    token: "your-bot-token"
    allow_from: []

tools:
  exec:
    timeout: 60
    restrict_to_workspace: true
  web:
    search:
      api_key: ""
  image:
    generation:
      api_key: ""  # Leave empty to use providers.qwen.api_key

agent_team:
  enabled: true  # Enable multi-agent collaboration
  # model: ""      # Optional: LLM model for task decomposition
```

### 2. Create Workspace Directory

```bash
mkdir -p ~/wukongbot-workspace/memory
mkdir -p ~/wukongbot-workspace/skills
mkdir -p ~/wukongbot-workspace
```

### 3. Run

```bash
# Direct chat
./wukongbot chat "Hello, wukongbot!"

# Start server with channels
./wukongbot run --config ~/.wukongbot/config.yaml
```

## Usage

### CLI Commands

```bash
# Version info
./wukongbot version

# Direct conversation
./wukongbot chat "What's the weather today?"

# Start server
./wukongbot run --config config.yaml

# With debug logging
./wukongbot run --debug

# Schedule command
./wukongbot schedule

# Skills management
./wukongbot skills list                    # List all available skills
./wukongbot skills list --workspace /path  # List skills from custom path
./wukongbot skills info github             # Show skill details
./wukongbot skills reload                  # Reload skills cache
```

## Configuration

### Complete Config Example

```yaml
agents:
  defaults:
    workspace: "~/wukongbot-workspace"
    model: "openrouter/google/gemini-2.5-pro"
    max_tokens: 8192
    temperature: 0.7
    max_tool_iterations: 10

providers:
  openrouter:
    api_key: "sk-or-xxx"
    api_base: ""

# Or use other providers:
# anthropic:
#   api_key: "sk-ant-api03-xxx"
# deepseek:
#   api_key: "sk-xxx"
# qwen:
#   api_key: "sk-xxx"

channels:
  telegram:
    enabled: true
    token: "123456789:xxxxxxx"
    allow_from: []  # Empty list means allow everyone

  whatsapp:
    enabled: false
    bridge_url: "ws://localhost:3001"
    allow_from: []

  feishu:
    enabled: false
    app_id: "cli_xxx"
    app_secret: "xxx"
    verification_token: "xxx"
    connection_mode: "websocket"  # "websocket" (default, recommended) or "webhook"
    allow_from: []

tools:
  web:
    search:
      api_key: ""  # Brave Search API Key for web_search
      max_results: 5
  exec:
    timeout: 120
    restrict_to_workspace: true
  image:
    generation:
      api_key: ""  # Leave empty to use providers.qwen.api_key
      api_base: "https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis"
      model: "wanx-v1"
    analysis:
      api_key: ""  # Leave empty to use providers.qwen.api_key
      api_base: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
      model: "qwen-vl-max"

swagger:
  max_endpoints: 50
  default_limit: 1000
  default_offset: 0
  sources: []

gateway:
  enabled: true
  host: "0.0.0.0"
  port: 8080

database:
  type: "sqlite"  # or "mysql"
  dsn: ""
```

### Environment Variable Overrides

You can override config values with environment variables:

```bash
export wukongbot_PROVIDERS__OPENROUTER__API_KEY="sk-or-xxx"
export wukongbot_AGENTS__DEFAULTS__MODEL="claude-3-5-sonnet"
export wukongbot_CHANNELS__TELEGRAM__ENABLED="true"

./wukongbot chat "Hello!"
```

## Tools Reference

### File Operations

| Tool | Description | Example |
|------|-------------|---------|
| `read_file` | Read file contents | `read_file(path="memory.md")` |
| `write_file` | Write to file | `write_file(path="notes.txt", content="hello")` |
| `edit_file` | Replace text in file | `edit_file(path="a.txt", old_text="foo", new_text="bar")` |
| `list_dir` | List directory contents | `list_dir(path=".")` |

### Shell Execution

| Tool | Description | Example |
|------|-------------|---------|
| `exec` | Execute shell command | `exec(command="ls -la")` |

Safety features:
- Dangerous commands blocked (rm -rf, format, etc.)
- Workspace restriction when enabled
- Command timeout (default 60 seconds)

### Web Tools

| Tool | Description | Example |
|------|-------------|---------|
| `web_search` | Search the web (requires Brave API key) | `web_search(query="Go programming tutorial")` |
| `web_fetch` | Fetch and parse web pages | `web_fetch(url="https://example.com")` |

### Image Tools

| Tool | Description | Example |
|------|-------------|---------|
| `generate_image` | Generate images from text | `generate_image(prompt="A cute cat", size="1024*1024")` |
| `analyze_image` | Analyze image content | `analyze_image(image_url="https://...", question="What is this?")` |

**generate_image Parameters:**
- `prompt` (required): Image description
- `size` (optional): Image size - "1024*1024", "768*1344", "864*1152", "720*1280", "480*960"
- `n` (optional): Number of images (1-4)

**analyze_image Parameters:**
- `image_url` (optional): Image URL
- `image_base64` (optional): Base64 encoded image data
- `image_mime` (optional): MIME type (e.g., "image/jpeg")
- `question` (optional): Question about the image

### Messaging

| Tool | Description | Example |
|------|-------------|---------|
| `message` | Send message to user | `message(channel="telegram", chat_id="123", content="Hello!")` |

### Subagents

| Tool | Description | Example |
|------|-------------|---------|
| `spawn` | Run task in background | `spawn(task="Analyze this codebase", label="Code Analysis")` |

### Cron Scheduling

wukongbot 支持两种定时任务模式：

#### 直接发送模式（简单通知）

适用于简单的提醒和通知，消息直接发送给用户，不经过LLM处理：

```
User: 每小时提醒我喝水
Bot: [调用 cron 工具]
cron(action="add", message="⏰ 喝水时间到啦！记得补充水分哦~ 🥤", every_seconds=3600, direct_send=true)
```

**特点**：
- 消息内容不变，直接发送给用户
- 适合简单的提醒、通知类任务
- 响应速度快，无LLM处理延迟

#### LLM处理模式（复杂任务）

适用于需要调用工具或二次处理的任务：

```
User: 每天早上8点提醒我带伞，需要查询天气
Bot: [调用 cron 工具]
cron(action="add", message="查询当前天气并提醒我带伞", cron_expr="0 0 8 * * *", direct_send=false)
```

**特点**：
- LLM会根据消息内容判断是否需要调用工具
- 消息包含结构化前缀【定时任务提醒】引导LLM处理
- 适合需要动态查询、数据获取的复杂任务

**工具示例**：

| Tool | Description | Example |
|------|-------------|---------|
| `cron` | Schedule reminders | `cron(action="add", message="Stand up!", every_seconds=3600)` |
| `cron` | List jobs | `cron(action="list")` |
| `cron` | Remove job | `cron(action="remove", job_id="abc123")` |

## Code Development Integration

The system integrates with external code development tools like **CC** and **Cursor** through an extensible hooks-based architecture.

### Configuration

Enable code development in your `config.yaml`:

```yaml
tools:
  hooks:
    code_development:
      enabled: true
      timeout: 300
      default_tool: "cursor"

      # Tool configurations
      executors:
        cursor:
          enabled: false
          command: "cursor"
          template: "cursor ai \"{task}\""
          check_command: "cursor --version"

      # Safety features
      auto_test: false
      test_command: "go test ./..."

      # Pre-write hooks
      pre_write:
        - name: "backup-before-write"
          type: "inline"

      # Progress tracking hooks
      task_hooks:
        - name: "progress-reporter"
        - name: "file-change-logger"
```

### Usage

Users can specify their preferred coding tool in natural language:

```bash
# Using Cursor
$ ./wukongbot chat "用cursor创建一个用户注册接口"
```

### Safety Features

- **Backup Before Write**: Automatically creates file backups (`.backup`) before modifications
- **Dangerous Command Check**: Blocks destructive commands (rm -rf, mkfs, mkdir, etc.)
- **Auto-Test**: Optional automatic test execution after writing Go files
- **Progress Tracking**: Real-time progress monitoring via hooks

### Adding New Tools

To add a new coding tool, implement the `ToolExecutor` interface in `internal/codedev/executor.go` and add configuration:

```go
type MyToolExecutor struct{}

func (e *MyToolExecutor) Name() string { return "mytool" }
func (e *MyToolExecutor) IsAvailable() bool { /* check installation */ }
func (e *MyToolExecutor) CheckCommand() string { return "mytool --version" }
```

## Demos

### Demo 1: File Operations and Memory

```bash
$ ./wukongbot chat "记住我的名字叫张三，存在memory文件里"

# wukongbot 会：
# 1. 写入 ~/wukongbot-workspace/memory/MEMORY.md
# 2. 返回确认

$ ./wukongbot chat "我的名字是什么？"

# wukongbot 会读取 memory 文件并回答
# "你的名字是张三"
```

### Demo 2: Shell Command Execution

```bash
$ ./wukongbot chat "查看当前目录的文件结构"

# wukongbot 会执行: list_dir(path=".")
# 返回类似:
# 📁 wukongbot-workspace/
# 📁 memory/
# 📁 skills/
# 📄 MEMORY.md

$ ./wukongbot chat "统计当前目录有多少个文件"

# wukongbot 会执行 shell 命令
# 返回: "当前目录有 15 个文件"
```

### Demo 3: Web Search

```bash
$ ./wukongbot chat "查找 Go 1.23 的新特性，搜索3条结果"

# wukongbot 调用 web_search
# 返回最新特性列表和链接

$ ./wukongbot chat "获取第一个链接的详细内容"

# wukongbot 调用 web_fetch
# 返回页面内容摘要
```

### Demo 4: Subagent Background Task

```bash
$ ./wukongbot chat "分析这个代码库的结构，生成摘要报告"

# wukongbot 会 spawn 一个子代理在后台运行
# 返回: "Spawned subagent 'Code Analysis' (id: abc123)"

# 子代理完成后会自动把结果发送到对话
```

### Demo 5: Schedule Reminder

```bash
$ ./wukongbot chat "每小时提醒我喝水"

# wukongbot 调用 cron 工具
# 返回: "Created job '每小时提醒我喝水' (id: xyz789)"

# 设置的间隔到达时会自动发送提醒消息
```

### Demo 6: Code Analysis and Writing

```bash
$ ./wukongbot chat "创建一个新的 Go 文件，实现一个 HTTP 服务器"

# wukongbot 会：
# 1. write_file 创建 server.go
# 2. 包含完整的代码

$ ./wukongbot chat "帮我优化这个代码"

# wukongbot 读取文件后提出优化建议
```

### Demo 7: Multi-turn Conversation with Tools

```bash
$ ./wukongbot chat "我需要帮助整理今天的笔记"
# 1. read_file 获取 MEMORY.md
# 2. write_file 更新今天的笔记
# 3. 提供整理建议

$ ./wukongbot chat "把这条记录添加到我的任务清单"
# 1. 编辑任务清单文件
# 2. 返回确认
```

### Demo 8: Image Generation

```bash
$ ./wukongbot chat "生成一张可爱的猫咪图片"

# wukongbot 调用 generate_image 工具
# 返回生成的图片 URL
```

### Demo 9: Image Analysis

```bash
$ ./wukongbot chat "分析这张图片的内容"  # 同时发送图片

# wukongbot 调用 analyze_image 工具
# 返回图片的描述和分析
```

## Skills System

Create skills in `~/wukongbot-workspace/skills/{skill-name}/SKILL.md`:

```markdown
---
name: "Custom Skill"
description: "What this skill does"
always: false
---

# Custom Skill

Describe the skill here...

## Usage

How to use the skill...
```

### Progressive Disclosure

Skills use a three-level loading system to manage context efficiently:

1. **Metadata (name + description)** - Always in context (~100 words)
2. **SKILL.md body** - When skill triggers (<5k words)
3. **Bundled resources** - As needed by the agent

**CRITICAL: How to Use Skills**

**YOU MUST use the read_file tool to load a skill BEFORE using it.** Do NOT attempt to use skills without first reading their SKILL.md file.

### Steps to use a skill:

1. **First**: Read the skill file using read_file:
   ```
   read_file(path="skills/{skill-name}/SKILL.md")
   ```

2. **Then**: Use the skill based on the guidance in the file

### Built-in Skills

wukongbot-go includes 10 built-in skills in the `skills/` directory:

| Skill | Description | Requirements |
|-------|-------------|--------------|
| `shell_helper` | Shell command assistance | None |
| `file_manager` | File operations help | None |
| `web_researcher` | Web search and fetch | None |
| `code_assistant` | Code writing and debugging | None |
| `skill-creator` | Create and design new AgentSkills | None |
| `github` | Interact with GitHub using `gh` CLI | `gh` |
| `weather` | Get weather forecasts (no API key) | `curl` |
| `summarize` | Summarize URLs, files, YouTube videos | `summarize` |
| `tmux` | Remote-control tmux sessions | `tmux` |
| `cron` | Schedule reminders and recurring tasks | None |

**Note:** Some skills have system requirements (bins). The agent will only show them as available when requirements are met.

### Skills Management Commands

```bash
# List all skills
./wukongbot skills list

# List skills from custom path
./wukongbot skills list --workspace /path/to/skills

# Show skill details
./wukongbot skills info skill-name

# Reload skills cache (for hot-reload)
./wukongbot skills reload

# Get skill size and token estimate
./wukongbot skills info github
# Output:
# Skill: github
#   Source: builtin
#   Size: 404 bytes (~0KB)
#   Tokens: ~101
#   Always Load: false
#   Description: Interact with GitHub using the `gh` CLI...
```

### Context Overflow Protection

To prevent context overflow, the following limits are enforced:

- **Max always-loaded skills**: 5 skills
- **Max always-loaded tokens**: 2000 tokens (~8000 characters)
- **Max single skill size**: 50KB

Skills that exceed these limits will be skipped and marked for on-demand loading.

### Hot Reload

New or modified skills are detected automatically when you run:
```bash
./wukongbot skills reload
```

This clears the skills cache and re-scans all skills directories.

## Swagger/OpenAPI Integration

wukongbot-go supports automatic generation of API tools from Swagger/OpenAPI specifications.

### Configuration

```yaml
swagger:
  max_endpoints: 50       # Maximum endpoints per API source
  default_limit: 1000     # Default page size for pagination
  default_offset: 0       # Default offset for pagination
  include_tags: []        # Only include endpoints with these tags
  exclude_tags: []        # Exclude endpoints with these tags
  sources:
    - id: example-api
      name: Example API
      url: https://api.example.com/swagger.json
      base_url: https://api.example.com
      enabled: true
      refresh_interval: 1h
      auth:
        type: bearer
        token: ${API_TOKEN}
```

### Authentication Types

- `bearer` - Bearer token authentication
- `basic` - Basic authentication (username/password)
- `apikey` - API key (sends X-API-Key header)
- `oauth2` - OAuth2 (not yet implemented)
- `none` - No authentication

### HTTP API for Swagger Management

When gateway is enabled, the following endpoints are available:

- `GET /api/swagger` - List Swagger sources and status
- `GET /api/swagger/tools` - List all API tools
- `POST /api/swagger/reload/:id` - Reload a specific Swagger source

## Agent Team

wukongbot-go supports multi-agent collaboration with automatic task decomposition and intelligent distribution to specialized agents. When you give a complex task, the Agent Team will:

1. **Decompose** the task into smaller, manageable subtasks using LLM
2. **Match** subtasks to specialized agents based on capabilities
3. **Execute** tasks in parallel (independent) or sequential (dependent) order
4. **Aggregate** results from all subtasks

### Features

- **LLM-based Task Decomposition**: Automatically breaks down complex tasks
- **Specialized Agents**: 4 pre-configured agents with different capabilities:
  - Frontend Specialist (JavaScript, TypeScript, React, Vue)
  - Backend Developer (Python, Go, FastAPI, Django)
  - Database Specialist (SQL, SQLAlchemy, GORM)
  - Testing Engineer (pytest, Jest, Cypres)
- **Hybrid Execution**: Parallel for independent tasks, sequential for dependent tasks
- **Capability Matching**: Intelligent agent selection based on language, domain, and tools
- **Load Balancing**: Distributes tasks across available agents

### Configuration

Enable the Agent Team in your `config.yaml`:

```yaml
agent_team:
  enabled: true  # Enable agent team feature
  model: ""      # Optional: LLM model for task decomposition (uses default if empty)
```

### Using Agent Team

The AI will automatically use the Agent Team when it detects complex tasks. Just describe what you need:

```
You: Develop an e-commerce website with user authentication

AI: [Automatically calls team_execute tool]

✅ **Agent Team 任务完成**

**任务**: Develop an e-commerce website with user authentication
**状态**: completed
**耗时**: 120 秒

**子任务详情**:
• Design Frontend Interface (completed): Frontend Specialist
  Result: Created homepage, product listing, shopping cart, checkout page...
• Implement Backend APIs (completed): Backend Developer
  Result: Built user authentication, product management, order processing APIs...
• Design Database Schema (completed): Database Specialist
  Result: Designed users, products, orders tables and relationships...
```

### Manual Control

You can also explicitly request agent team usage:

```
Please use the agent team to break down this project into subtasks and execute them.
```

### How It Works

```
User Request → LLM detects complex task
                    ↓
            Calls team_execute tool
                    ↓
          TaskDecomposer (LLM)
                    ↓
            Breaks into 3-5 subtasks
                    ↓
         TaskCoordinator analyzes dependencies
                    ↓
         ┌──────────────┬──────────────┐
         │              │              │
    Independent   Dependent A → Dependent B → Dependent C
    (parallel)     (sequential)
         │              │              │
         ↓              ↓              ↓
    Assigned to    Assigned to    Assigned to
    Agent #1       Agent #2       Agent #3
         │              │              │
         └───────────────┴──────────────┘
                    ↓
              Aggregated Result
                    ↓
               Report to User
```

## Hooks System

wukongbot-go supports an event-driven hooks system inspired by Claude Code, allowing custom behaviors at specific lifecycle events.

### Hook Events

| Event | Description | Timing |
|-------|-------------|--------|
| `PreToolUse` | Before tool execution | Can deny/modify tool execution |
| `PostToolUse` | After successful tool execution | Can process results |
| `PostToolUseFailure` | After failed tool execution | Can handle errors |
| `SessionStart` | When a session starts | Initialization tasks |
| `SessionEnd` | When a session ends | Cleanup tasks |
| `UserPromptSubmit` | When user submits a prompt | Task delegation |

### Code Development Hooks

Specialized hooks for coding workflows with built-in safety features:

| Hook | Function |
|------|----------|
| `backup-before-write` | Automatically backup files before writing |
| `dangerous-command-check` | Block dangerous commands (rm -rf, format, etc.) |
| `auto-test` | Automatically run tests after writing Go files |
| `code-development-task` | Delegate coding tasks to external tools |

### Configuration

```yaml
tools:
  hooks:
    enabled: true          # Enable hooks system globally
    timeout: 10            # Hook execution timeout (seconds)

    # Custom hooks
    pre_tool_use:
      - name: "dangerous-command-check"
        matcher: "exec"
        type: "inline"

    post_tool_use:
      - name: "log-write"
        matcher: "write_file"
        type: "command"
        command: "echo 'File written: {tool_input.path}' >> hooks.log"
```

### Usage Example

When `code_development` hooks are enabled:

```bash
$ ./wukongbot chat "写一个 hello world 函数到 main.go"

# System will:
# 1. Create backup: main.go.backup
# 2. Write the file
# 3. Auto-run tests: ✅ Tests passed
```

Dangerous commands are automatically blocked:

```bash
$ ./wukongbot chat "删除所有文件"

# System response:
# Dangerous command blocked: rm -rf
```

## Tmux Interactive Cards

### Overview

Tmux Interactive Cards provide a safe confirmation mechanism for dangerous command execution and interactive questions. When users attempt dangerous operations, the system sends Feishu interactive cards requiring user confirmation before proceeding.

### Key Features

- **Dangerous Command Detection**: Automatically identifies 13 dangerous command patterns (rm -rf, dd, mkfs, shutdown, etc.)
- **Interactive Question Detection**: Detects 14 interactive question patterns (confirm, select, input, etc.)
- **Feishu Card Interaction**: Implements user confirmation and input via Feishu interactive cards
- **Session Management**: 5-minute timeout, thread-safe, automatic cleanup
- **Comprehensive Logging**: Detailed logging for all operations

### Use Cases

1. **Dangerous Command Confirmation**
   ```
   User: Execute rm -rf /tmp/test
   System: Sends confirmation card "⚠️ Dangerous Command Confirmation"
   User: Clicks "Confirm" button
   System: Continues command execution
   ```

2. **Interactive Question Handling**
   ```
   System: Detects output containing "Are you sure? (y/n)"
   System: Sends interactive card
   User: Selects answer
   System: Sends answer to tmux session
   ```

### Workflow

```
User sends command
    ↓
TmuxInteractTool.BeforeExecute()
    ↓
Dangerous command detected?
    ├─ Yes → Generate confirmation card → Send to user
    │         ↓
    │    User clicks button
    │         ↓
    │    Feishu callback POST /feishu/card/callback
    │         ↓
    │    EnhancedHandler.HandleToolCardCallback()
    │         ↓
    │    Continue tool execution
    │
    └─ No → Execute tool directly
         ↓
    TmuxInteractTool.AfterExecute()
         ↓
    Interactive question detected?
         ├─ Yes → Send interactive card
         └─ No → Return result
```

### HTTP Endpoint

**Card Callback Endpoint**: `POST /feishu/card/callback`

Receives Feishu card callbacks and processes user answers:

```bash
curl -X POST http://localhost:8080/feishu/card/callback \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-123",
    "user_id": "user-456",
    "action": "button_click",
    "value": {"confirm": "yes"}
  }'
```

### Supported Card Types

| Type | Description | Use Case |
|------|-------------|----------|
| `confirm` | Confirmation card | Dangerous command confirmation |
| `single_choice` | Single choice card | Select one option |
| `multiple_choice` | Multiple choice card | Select multiple options |
| `input` | Input card | Text input |

### Documentation

- **Usage Guide**: [docs/tmux-interactive-cards-usage.md](docs/tmux-interactive-cards-usage.md)
- **API Documentation**: [docs/feishu-card-callback.md](docs/feishu-card-callback.md)
- **Design Document**: [docs/plans/2026-03-07-tmux-card-design.md](docs/plans/2026-03-07-tmux-card-design.md)

---

## HTTP API Endpoints

When gateway is enabled in config, wukongbot provides HTTP endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check endpoint |
| `/feishu/events` | GET/POST | Feishu webhook handler |
| `/api/skills` | GET | List all available skills |
| `/api/skills` | POST | Reload skills cache |
| `/api/swagger` | GET | List Swagger sources and status |
| `/api/swagger/tools` | GET | List all API tools from Swagger |
| `/api/swagger/reload/:id` | POST | Reload a specific Swagger source |

## Feishu Channel Configuration

### Connection Modes

wukongbot-go supports two Feishu connection modes:

| Mode | Description | Advantages | Disadvantages |
|------|-------------|------------|----------------|
| **WebSocket** (default) | Long connection, active connect to Feishu server | No public IP needed, simple setup | Requires stable connection |
| **Webhook** | HTTP callback, Feishu pushes actively | Immediate response | Requires public IP/tunnel |

### WebSocket Mode (Recommended)

1. **Create Feishu App**
   - Visit https://open.feishu.cn/
   - Create enterprise custom app
   - Enable "Robot capability"
   - Add permissions: `im:message`, `im:message:send_at_bot`

2. **Configure Event Subscription**
   - Go to "Event Subscription" → "Event Configuration"
   - Select "Long connection" mode
   - Add event: `im.message.receive_v1`
   - No need to configure Request URL

3. **Get Credentials**
   - On "Credentials & Basic Info" page:
     - **App ID** (cli_xxx)
     - **App Secret**
     - **Verification Token** (if available)

4. **Configuration File**
   ```yaml
   channels:
     feishu:
       enabled: true
       app_id: "cli_a90308a3f679dcc4"
       app_secret: "8YuPQoUs00l2OlLrMMmXPeXZ0O3kNKyy"
       connection_mode: "websocket"
       allow_from: []  # Allow all users
   ```

5. **Start Service**
   ```bash
   ./wukongbot run --config config.yaml
   ```

### Webhook Mode

If you need to use Webhook mode, you need public IP or internal network tunnel:

1. Configure server's public address
2. Set Request URL in Feishu Open Platform: `http://your-server:12345/feishu/events`
3. Configuration file:
   ```yaml
   channels:
     feishu:
       enabled: true
       connection_mode: "webhook"
       verification_token: "your-verification-token"
   ```

4. Ensure HTTP gateway is enabled:
   ```yaml
   gateway:
     enabled: true
     host: "0.0.0.0"
     port: 12345
   ```

## Bootstrap Files (Optional)

Create these files in your workspace to customize agent behavior:

- `AGENTS.md` - Agent configuration
- `SOUL.md` - Personality/character definition
- `USER.md` - User-specific instructions
- `TOOLS.md` - Tool usage guidelines
- `IDENTITY.md` - Core identity configuration

## Project Structure

```
wukongbot-go/
├── cmd/wukongbot/          # CLI entry point
├── internal/
│   ├── agent/            # Core agent logic
│   │   ├── loop.go       # Main processing loop
│   │   ├── context.go    # Context builder
│   │   ├── memory.go     # Memory system
│   │   ├── skills.go     # Skills loader
│   │   └── subagent.go   # Subagent manager
│   ├── bus/              # Message bus
│   ├── channels/         # Channel implementations
│   │   ├── telegram.go
│   │   ├── whatsapp.go
│   │   └── feishu.go
│   ├── config/           # Configuration
│   ├── cron/             # Cron service
│   ├── providers/        # LLM providers
│   │   ├── factory.go
│   │   ├── openai.go
│   │   ├── anthropic.go
│   │   └── openrouter.go
│   ├── session/          # Storage (SQLite/MySQL)
│   └── swagger/          # Swagger/OpenAPI integration
│       ├── config.go
│       ├── parser.go
│       ├── generator.go
│       ├── client.go
│       └── registry.go
├── tools/                # Tool implementations
│   ├── filesystem.go
│   ├── shell.go
│   ├── web.go
│   ├── message.go
│   ├── spawn.go
│   ├── cron.go
│   └── image.go
└── skills/               # Built-in skills
```

## Development

### Run Tests

```bash
go test ./...
```

### Add New Channel

```go
// internal/channels/mychannel.go
package channels

type MyChannel struct {
    // ...
}

func NewMyChannel(cfg MyConfig) *MyChannel {
    // ...
}

func (c *MyChannel) Start(ctx context.Context) error {
    // ...
}
```

### Add New Provider

```go
// internal/providers/mine.go
type MyProvider struct {
    client *http.Client
    // ...
}

func NewMyProvider(apiKey, model string) providers.LLMProvider {
    // ...
}
```

## Docker

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o wukongbot ./cmd/wukongbot

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/wukongbot /usr/local/bin/
WORKDIR /root
VOLUME ["/root/wukongbot-workspace"]
EXPOSE 8080
CMD ["wukongbot", "run"]
```

```bash
docker build -t wukongbot .
docker run -v ~/wukongbot-workspace:/root/wukongbot-workspace wukongbot
```

## License

MIT License

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch

## TODO

- [ ] Implement browser operation support
- [ ] Self-improvement and bug fixing capabilities
- [ ] Self-upgrade and hot reload support
- [ ] Session compression
- [ ] Smart skills management
5. Create a Pull Request