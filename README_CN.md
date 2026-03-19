# wukongbot-go

轻量级 AI 助手框架，使用 Go 语言编写。支持多渠道（Telegram、WhatsApp、飞书）和多种 LLM 提供商。

[![CI](https://github.com/konglong87/wukongbot/actions/workflows/ci.yml/badge.svg)](https://github.com/konglong87/wukongbot/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/konglong87/wukongbot)](https://goreportcard.com/report/github.com/konglong87/wukongbot)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)


> **中文文档**: [README_CN.md](README_CN.md) | **English Documentation**: [README.md](README.md)

## 功能特性

- **多渠道支持**: WhatsApp、飞书
- **多 LLM 支持**: OpenAI、Anthropic、OpenRouter、DeepSeek、Gemini、Groq、智谱、vLLM 等
- **工具系统**: 文件操作、Shell 命令、网页搜索、子代理、定时任务、文生图、图生文
- **Hooks 系统**: 事件驱动的钩子系统，支持在生命周期事件（PreToolUse、PostToolUse、代码开发等）中自定义行为
- **Tmux 交互式卡片**: 🆕 危险命令确认、交互式问题检测、飞书卡片交互、会话管理
- **持久化内存**: SQLite 或 MySQL 存储 + 文件内存
- **技能系统**: Markdown 格式技能定义，支持 YAML 前置元数据，渐进式加载
- **子代理**: 后台任务执行，结果自动通知
- **定时调度**: 定时发送消息和周期性任务
- **Swagger/OpenAPI 集成**: 从 OpenAPI 规范自动生成 API 工具
- **图片功能**: 使用通义万相和 Qwen-VL 模型实现文生图和图生文
- **Agent Team**: 多 Agent 协作，支持自动任务分解和智能分发

## 安装

### 从源码编译

```bash
git clone https://github.com/konglong87/wukongbot.git
cd wukongbot
go build -o wukongbot ./cmd/wukongbot
```

### 构建不同平台版本

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o wukongbot-linux-amd64 ./cmd/wukongbot

# macOS
GOOS=darwin GOARCH=arm64 go build -o wukongbot-arm64 ./cmd/wukongbot
GOOS=darwin GOARCH=amd64 go build -o wukongbot-amd64 ./cmd/wukongbot

# Windows
GOOS=windows GOARCH=amd64 go build -o wukongbot.exe ./cmd/wukongbot
```

## 快速开始

### 1. 创建配置文件

创建 `~/.wukongbot/config.yaml`：

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
    api_key: "sk-xxx"  # 替换为你的 API Key

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
      api_key: ""  # 留空则使用 providers.qwen.api_key

agent_team:
  enabled: true  # 启用多 agent 协作
  # model: ""      # 可选：用于任务分解的 LLM 模型
```

### 2. 创建工作目录

```bash
mkdir -p ~/wukongbot-workspace/memory
mkdir -p ~/wukongbot-workspace/skills
mkdir -p ~/wukongbot-workspace
```

### 3. 运行

```bash
# 直接对话
./wukongbot chat "你好，wukongbot！"

# 启动服务（连接 Telegram 等渠道）
./wukongbot run --config ~/.wukongbot/config.yaml
```

## 使用方法

### CLI 命令

```bash
# 查看版本
./wukongbot version

# 直接对话
./wukongbot chat "今天天气怎么样？"

# 启动服务
./wukongbot run --config config.yaml

# 调试模式（更多日志）
./wukongbot run --debug

# 定时命令
./wukongbot schedule

# 技能管理
./wukongbot skills list                    # 列出所有可用技能
./wukongbot skills list --workspace /path  # 从自定义路径列出技能
./wukongbot skills info github             # 显示技能详情
./wukongbot skills reload                  # 重新加载技能缓存
```

## 配置文件

### 完整配置示例

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

# 或使用其他提供商:
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
    allow_from: []  # 空列表表示允许所有人

  whatsapp:
    enabled: false
    bridge_url: "ws://localhost:3001"
    allow_from: []

  feishu:
    enabled: false
    app_id: "cli_xxx"
    app_secret: "xxx"
    encrypt_key: ""  # 可选，用于加密验证
    verification_token: "xxx"
    connection_mode: "websocket"  # "websocket" (默认，推荐，无需公网IP) 或 "webhook" (需要公网IP)
    allow_from: []

tools:
  web:
    search:
      api_key: ""  # Brave Search API Key
      max_results: 5
  exec:
    timeout: 120
    restrict_to_workspace: true
  image:
    generation:
      api_key: ""  # 留空则使用 providers.qwen.api_key
      api_base: "https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis"
      model: "wanx-v1"
    analysis:
      api_key: ""  # 留空则使用 providers.qwen.api_key
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
  type: "sqlite"  # 或 "mysql"
  dsn: ""
```

### 环境变量覆盖

你可以使用环境变量覆盖配置：

```bash
export wukongbot_PROVIDERS__OPENROUTER__API_KEY="sk-or-xxx"
export wukongbot_AGENTS__DEFAULTS__MODEL="claude-3-5-sonnet"
export wukongbot_CHANNELS__TELEGRAM__ENABLED="true"

./wukongbot chat "你好！"
```

## 工具参考

### 文件操作

| 工具 | 说明 | 示例 |
|------|------|------|
| `read_file` | 读取文件内容 | `read_file(path="memory.md")` |
| `write_file` | 写入文件 | `write_file(path="notes.txt", content="内容")` |
| `edit_file` | 替换文件中的文本 | `edit_file(path="a.txt", old_text="旧", new_text="新")` |
| `list_dir` | 列出目录内容 | `list_dir(path=".")` |

### Shell 执行

| 工具 | 说明 | 示例 |
|------|------|------|
| `exec` | 执行 Shell 命令 | `exec(command="ls -la")` |

安全特性：
- 危险命令被阻止（rm -rf、format 等）
- 启用时限制工作目录
- 命令超时（默认 60 秒）

### 网页工具

| 工具 | 说明 | 示例 |
|------|------|------|
| `web_search` | 网页搜索（需要 Brave API key） | `web_search(query="Go 语言教程")` |
| `web_fetch` | 获取并解析网页 | `web_fetch(url="https://example.com")` |

### 图片工具

| 工具 | 说明 | 示例 |
|------|------|------|
| `generate_image` | 文生图 | `generate_image(prompt="一只可爱的猫", size="1024*1024")` |
| `analyze_image` | 图生文 | `analyze_image(image_url="https://...", question="这是什么？")` |

**generate_image 参数：**
- `prompt` (必需): 图片描述
- `size` (可选): 图片尺寸 - "1024*1024", "768*1344", "864*1152", "720*1280", "480*960"
- `n` (可选): 生成数量 (1-4)

**analyze_image 参数：**
- `image_url` (可选): 图片 URL
- `image_base64` (可选): Base64 编码的图片数据
- `image_mime` (可选): MIME 类型（如 "image/jpeg"）
- `question` (可选): 针对图片的问题

### 消息发送

| 工具 | 说明 | 示例 |
|------|------|------|
| `message` | 发送消息给用户 | `message(channel="telegram", chat_id="123", content="你好！")` |

### 子代理

| 工具 | 说明 | 示例 |
|------|------|------|
| `spawn` | 后台运行任务 | `spawn(task="分析代码库", label="代码分析")` |

### 定时任务

| 工具 | 说明 | 示例 |
|------|------|------|
| `cron` | 设置提醒 | `cron(action="add", message="喝水", every_seconds=3600)` |
| `cron` | 列出任务 | `cron(action="list")` |
| `cron` | 删除任务 | `cron(action="remove", job_id="abc123")` |

## 代码开发集成

系统通过基于 hooks 的可扩展架构，与外部代码开发工具（如 **CC** 和 **Cursor**）集成。

### 配置

在 `config.yaml` 中启用代码开发功能：

```yaml
tools:
  hooks:
    code_development:
      enabled: true
      timeout: 300
      default_tool: "cursor"

      # 工具配置
      executors:
        cursor:
          enabled: false
          command: "cursor"
          template: "cursor ai \"{task}\""
          check_command: "cursor --version"

      # 安全功能
      auto_test: false
      test_command: "go test ./..."

      # 写入前 Hooks
      pre_write:
        - name: "backup-before-write"
          type: "inline"

      # 进度跟踪 Hooks
      task_hooks:
        - name: "progress-reporter"
        - name: "file-change-logger"
```

### 使用方式

用户可以在自然语言中指定编码工具：

```bash
# 使用 Cursor
$ ./wukongbot chat "用cursor创建一个用户注册接口"
```

### 安全特性

- **写前备份**: 文件修改前自动创建备份（`.backup`）
- **危险命令检查**: 阻止破坏性命令（rm -rf, mkfs, mkdir 等）
- **自动测试**: 可选的写完 Go 文件后自动运行测试
- **进度跟踪**: 通过 hooks 实时监控进度

### 添加新工具

要添加新的编码工具，需要在 `internal/codedev/executor.go` 中实现 `ToolExecutor` 接口并添加配置：

```go
type MyToolExecutor struct{}

func (e *MyToolExecutor) Name() string { return "mytool" }
func (e *MyToolExecutor) IsAvailable() bool { /* 检查是否安装 */ }
func (e *MyToolExecutor) CheckCommand() string { return "mytool --version" }
```

## 演示 Demo

### Demo 1: 文件操作和记忆

```bash
$ ./wukongbot chat "记住我的名字叫恐龙，存在 memory 文件里"

# wukongbot 会：
# 1. 写入 ~/wukongbot-workspace/memory/MEMORY.md
# 2. 返回确认

$ ./wukongbot chat "我的名字是什么？"

# wukongbot 会读取 memory 文件并回答
# "你的名字是恐龙"
```

### Demo 2: Shell 命令执行

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

### Demo 3: 网页搜索

```bash
$ ./wukongbot chat "搜索 Go 1.23 的新特性，返回3条结果"

# wukongbot 调用 web_search
# 返回最新特性列表和链接

$ ./wukongbot chat "获取第一个链接的详细内容"

# wukongbot 调用 web_fetch
# 返回页面内容摘要
```

### Demo 4: 子代理后台任务

```bash
$ ./wukongbot chat "分析这个代码库的结构，生成摘要报告"

# wukongbot 会 spawn 一个子代理在后台运行
# 返回: "Spawned subagent '代码分析' (id: abc123)"

# 子代理完成后会自动把结果发送到对话
```

### Demo 5: 设置定时提醒

```bash
$ ./wukongbot chat "每小时提醒我喝水"

# wukongbot 调用 cron 工具
# 返回: "Created job '每小时提醒我喝水' (id: xyz789)"

# 设置的时间到达时会自动发送提醒消息
```

### Demo 6: 代码编写

```bash
$ ./wukongbot chat "创建一个新的 Go 文件，实现一个 HTTP 服务器"

# wukongbot 会：
# 1. write_file 创建 server.go
# 2. 包含完整的代码

$ ./wukongbot chat "帮我的代码添加单元测试"

# wukongbot 读取文件后添加测试代码
```

### Demo 7: 多轮对话 + 工具调用

```bash
$ ./wukongbot chat "我需要帮助整理今天的笔记"
# 1. read_file 获取 MEMORY.md
# 2. write_file 更新今天的笔记
# 3. 提供整理建议

$ ./wukongbot chat "把这条记录添加到我的待办清单"
# 1. 编辑待办清单文件
# 2. 返回确认

$ ./wukongbot chat "显示我的待办清单"
# 1. read_file 读取待办文件
# 2. 以清晰格式展示
```

### Demo 8: 文生图

```bash
$ ./wukongbot chat "生成一张可爱的猫咪图片"

# wukongbot 调用 generate_image 工具
# 返回生成的图片 URL
```

### Demo 9: 图生文

```bash
$ ./wukongbot chat "分析这张图片的内容"  # 同时发送图片

# wukongbot 调用 analyze_image 工具
# 返回图片的描述和分析
```

## 技能系统

在 `~/wukongbot-workspace/skills/{技能名}/SKILL.md` 创建技能：

```markdown
---
name: "自定义技能"
description: "这个技能的功能"
always: false
---

# 自定义技能

在这里描述技能...

## 使用方法

如何使用这个技能...
```

### 渐进式加载

技能使用三级加载系统来高效管理上下文：

1. **元数据（名称 + 描述）** - 始终在上下文中（约 100 词）
2. **SKILL.md 内容** - 技能触发时加载（<5k 词）
3. **捆绑资源** - 按需加载

**重要：如何使用技能**

**你必须在使用技能之前使用 read_file 工具加载它。** 不要尝试在不先读取其 SKILL.md 文件的情况下使用技能。

### 使用技能的步骤：

1. **首先**：使用 read_file 读取技能文件：
   ```
   read_file(path="skills/{技能名}/SKILL.md")
   ```

2. **然后**：根据文件中的指导使用技能

### 内置技能

wukongbot-go 在 `skills/` 目录中包含 10 个内置技能：

| 技能 | 说明 | 要求 |
|------|------|------|
| `shell_helper` | Shell 命令帮助 | 无 |
| `file_manager` | 文件操作帮助 | 无 |
| `web_researcher` | 网页搜索和抓取 | 无 |
| `code_assistant` | 代码编写和调试 | 无 |
| `skill-creator` | 创建和设计新 AgentSkills | 无 |
| `github` | 使用 `gh` CLI 与 GitHub 交互 | `gh` |
| `weather` | 获取天气预报（无需 API key） | `curl` |
| `summarize` | 摘要 URL、文件、YouTube 视频 | `summarize` |
| `tmux` | 远程控制 tmux 会话 | `tmux` |
| `cron` | 安排提醒和周期性任务 | 无 |

**注意：** 部分技能有系统要求（依赖命令）。代理只会在满足要求时将它们显示为可用。

### 技能管理命令

```bash
# 列出所有技能
./wukongbot skills list

# 从自定义路径列出技能
./wukongbot skills list --workspace /path/to/skills

# 显示技能详情
./wukongbot skills info skill-name

# 重新加载技能缓存（用于热重载）
./wukongbot skills reload

# 获取技能大小和 token 估算
./wukongbot skills info github
# 输出：
# Skill: github
#   Source: builtin
#   Size: 404 bytes (~0KB)
#   Tokens: ~101
#   Always Load: false
#   Description: 使用 `gh` CLI 与 GitHub 交互...
```

### 上下文溢出保护

为了防止上下文溢出，执行以下限制：

- **最大常驻技能数**：5 个技能
- **最大常驻 token 数**：2000 tokens（约 8000 字符）
- **单个技能最大大小**：50KB

超过这些限制的技能将被跳过并标记为按需加载。

### 热重载

运行以下命令时自动检测新的或修改的技能：
```bash
./wukongbot skills reload
```

这会清除技能缓存并重新扫描所有技能目录。

## Swagger/OpenAPI 集成

wukongbot-go 支持从 Swagger/OpenAPI 规范自动生成 API 工具。

### 配置

```yaml
swagger:
  max_endpoints: 50       # 每个 API 源的最大端点数
  default_limit: 1000     # 分页的默认页面大小
  default_offset: 0       # 默认偏移量
  include_tags: []        # 仅包含带有这些标签的端点
  exclude_tags: []        # 排除带有这些标签的端点
  sources:
    - id: example-api
      name: 示例 API
      url: https://api.example.com/swagger.json
      base_url: https://api.example.com
      enabled: true
      refresh_interval: 1h
      auth:
        type: bearer
        token: ${API_TOKEN}
```

### 认证类型

- `bearer` - Bearer 令牌认证
- `basic` - 基本认证（用户名/密码）
- `apikey` - API 密钥（发送 X-API-Key 头）
- `oauth2` - OAuth2（尚未实现）
- `none` - 无认证

### Swagger 管理的 HTTP API

启用 gateway 时，可以使用以下端点：

- `GET /api/swagger` - 列出 Swagger 源及其状态
- `GET /api/swagger/tools` - 列出所有来自 Swagger 的 API 工具
- `POST /api/swagger/reload/:id` - 重新加载特定的 Swagger 源

## Agent Team

wukongbot-go 支持多 Agent 协作，具备自动任务分解和智能分发到专业 Agent 的能力。当您给出复杂任务时，Agent Team 将：

1. **分解** 任务为更小的、可管理的子任务（使用 LLM）
2. **匹配** 子任务到基于能力的专业 Agent
3. **执行** 任务：并行（独立）或串行（依赖）执行
4. **聚合** 来自所有子任务的结果

### 功能特性

- **基于 LLM 的任务分解**：自动将复杂任务拆解
- **专业 Agent**：4 个预配置的具有不同能力的 Agent：
  - 前端专家（JavaScript、TypeScript、React、Vue）
  - 后端开发者（Python、Go、FastAPI、Django）
  - 数据库专家（SQL、SQLAlchemy、GORM）
  - 测试工程师（pytest、Jest、Cypress）
- **混合执行**：独立任务并行执行，依赖任务串行执行
- **能力匹配**：基于语言、领域和工具的智能 Agent 选择
- **负载均衡**：在可用 Agent 之间分发任务

### 配置

在 `config.yaml` 中启用 Agent Team：

```yaml
agent_team:
  enabled: true  # 启用 agent team 功能
  # model: ""      # 可选：用于任务分解的 LLM 模型（留空则使用默认模型）
```

### 使用 Agent Team

当 AI 检测到复杂任务时，会自动使用 Agent Team。只需描述您的需求：

```
你: 开发一个带用户认证的电商平台

AI: [自动调用 team_execute 工具]

✅ **Agent Team 任务完成**

**任务**: 开发一个带用户认证的电商平台
**状态**: completed
**耗时**: 120 秒

**子任务详情**:
• 设计前端界面 (completed): Frontend Specialist
  结果: 已完成电商首页、商品列表、购物车、结账页面...
• 实现后端 API (completed): Backend Developer
  结果: 已构建用户认证、商品管理、订单处理 API...
• 设计数据库架构 (completed): Database Specialist
  结果: 已设计 users、products、orders 表和关系...
```

### 手动控制

您也可以显式请求使用 agent team：

```
请使用 agent team 将这个项目分解为子任务并执行。
```

### 工作原理

```
用户请求 → LLM 检测复杂任务
                    ↓
            调用 team_execute 工具
                    ↓
          TaskDecomposer (LLM)
                    ↓
            分解为 3-5 个子任务
                    ↓
       TaskCoordinator 分析依赖关系
                    ↓
         ┌──────────────┬──────────────┐
         │              │              │
    独立任务       依赖任务 A → 依赖任务 B → 依赖任务 C
   (并行执行)      (串行执行)
         │              │              │
         ↓              ↓              ↓
    分配给        分配给        分配给
   Agent #1      Agent #2      Agent #3
         │              │              │
         └───────────────┴──────────────┘
                    ↓
              聚合结果
                    ↓
               向用户报告
```

## Hooks 系统

wukongbot-go 支持受 Claude Code 启发的基于事件的钩子系统，允许在特定生命周期事件中自定义行为。

### Hook 事件

| 事件 | 说明 | 触发时机 |
|------|------|----------|
| `PreToolUse` | 工具执行前 | 可以拒绝/修改工具执行 |
| `PostToolUse` | 工具执行成功后 | 可以处理结果 |
| `PostToolUseFailure` | 工具执行失败后 | 可以处理错误 |
| `SessionStart` | 会话开始时 | 初始化任务 |
| `SessionEnd` | 会话结束时 | 清理任务 |
| `UserPromptSubmit` | 用户提交提示时 | 任务委托 |

### 代码开发 Hooks

专为编码工作流设计的专用 hooks，内置安全功能：

| Hook | 功能 |
|------|------|
| `backup-before-write` | 写入文件前自动备份 |
| `dangerous-command-check` | 阻止危险命令（rm -rf、format 等） |
| `auto-test` | 写入 Go 文件后自动运行测试 |
| `code-development-task` | 将编码任务委托给外部工具 |

### 配置

```yaml
tools:
  hooks:
    enabled: true          # 全局启用 hooks 系统
    timeout: 10            # hooks 执行超时时间（秒）

    # 自定义 hooks
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

### 使用示例

启用 `code_development` hooks 后：

```bash
$ ./wukongbot chat "写一个 hello world 函数到 main.go"

# 系统会：
# 1. 创建备份: main.go.backup
# 2. 写入文件
# 3. 自动运行测试: ✅ Tests passed
```

危险命令会被自动阻止：

```bash
$ ./wukongbot chat "删除所有文件"

# 系统响应:
# Dangerous command blocked: rm -rf
```

## Tmux 交互式卡片

### 功能简介

Tmux 交互式卡片功能为危险命令执行和交互式问题提供了安全确认机制。当用户尝试执行危险操作时，系统会发送飞书交互式卡片，要求用户确认后才继续执行。

### 核心特性

- **危险命令检测**: 自动识别 13 种危险命令模式（rm -rf、dd、mkfs、shutdown 等）
- **交互式问题检测**: 检测 14 种交互式问题模式（确认、选择、输入等）
- **飞书卡片交互**: 通过飞书交互式卡片实现用户确认和输入
- **会话管理**: 5 分钟超时、并发安全、自动清理
- **完整日志**: 所有操作都有详细日志记录

### 使用场景

1. **危险命令确认**
   ```
   用户: 执行 rm -rf /tmp/test
   系统: 发送确认卡片 "⚠️ 危险命令确认"
   用户: 点击"确认"按钮
   系统: 继续执行命令
   ```

2. **交互式问题处理**
   ```
   系统: 检测到输出包含 "Are you sure? (y/n)"
   系统: 发送交互式卡片
   用户: 选择答案
   系统: 将答案发送到 tmux 会话
   ```

### 工作流程

```
用户发送命令
    ↓
TmuxInteractTool.BeforeExecute()
    ↓
检测到危险命令？
    ├─ 是 → 生成确认卡片 → 发送给用户
    │         ↓
    │    用户点击按钮
    │         ↓
    │    飞书回调 POST /feishu/card/callback
    │         ↓
    │    EnhancedHandler.HandleToolCardCallback()
    │         ↓
    │    继续执行工具
    │
    └─ 否 → 直接执行工具
         ↓
    TmuxInteractTool.AfterExecute()
         ↓
    检测到交互式问题？
         ├─ 是 → 发送交互式卡片
         └─ 否 → 返回结果
```

### HTTP 端点

**卡片回调端点**: `POST /feishu/card/callback`

接收飞书卡片回调，处理用户答案：

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

### 配置示例

```yaml
# config.yaml
gateway:
  enabled: true
  host: "0.0.0.0"
  port: 8080

# 飞书渠道配置
channels:
  feishu:
    enabled: true
    app_id: "cli_xxx"
    app_secret: "xxx"
    # 卡片回调地址：http://your-domain:8080/feishu/card/callback
```

### 支持的卡片类型

| 类型 | 说明 | 使用场景 |
|------|------|---------|
| `confirm` | 确认卡片 | 危险命令确认 |
| `single_choice` | 单选卡片 | 选择一个选项 |
| `multiple_choice` | 多选卡片 | 选择多个选项 |
| `input` | 输入卡片 | 文本输入 |

### 详细文档

- **使用指南**: [docs/tmux-interactive-cards-usage.md](docs/tmux-interactive-cards-usage.md)
- **API 文档**: [docs/feishu-card-callback.md](docs/feishu-card-callback.md)
- **设计文档**: [docs/plans/2026-03-07-tmux-card-design.md](docs/plans/2026-03-07-tmux-card-design.md)

---

## HTTP API 端点

在配置中启用 gateway 时，wukongbot 提供以下 HTTP 端点：

| 端点 | 方法 | 说明 |
|----------|--------|-------------|
| `/health` | GET | 健康检查端点 |
| `/feishu/events` | GET/POST | 飞书 Webhook 处理器 |
| `/api/skills` | GET | 列出所有可用技能 |
| `/api/skills` | POST | 重新加载技能缓存 |
| `/api/swagger` | GET | 列出 Swagger 源及其状态 |
| `/api/swagger/tools` | GET | 列出所有来自 Swagger 的 API 工具 |
| `/api/swagger/reload/:id` | POST | 重新加载特定的 Swagger 源 |

## 飞书渠道配置

### 连接模式

wukongbot 支持两种飞书连接模式：

| 模式 | 说明 | 优点 | 缺点 |
|------|------|------|------|
| **WebSocket** (默认) | 长连接，主动连接飞书服务器 | 无需公网IP，配置简单 | 需要保持连接稳定 |
| **Webhook** | HTTP回调，飞书主动推送 | 响应即时 | 需要公网IP/内网穿透 |

### WebSocket 长连接模式（推荐）

1. **创建飞书应用**
   - 访问 https://open.feishu.cn/
   - 创建企业自建应用
   - 启用「机器人能力」
   - 添加权限：`im:message`、`im:message:send_at_bot`

2. **配置事件订阅**
   - 进入「事件订阅」→「事件配置」
   - 选择「长连接」模式
   - 添加事件：`im.message.receive_v1`
   - 无需配置 Request URL

3. **获取凭证**
   - 在「凭证与基础信息」页面获取：
     - **App ID** (cli_xxx)
     - **App Secret**
     - **Verification Token** （如果有）

4. **配置文件**
   ```yaml
   channels:
     feishu:
       enabled: true
       app_id: "cli_a90308a3f679dcc4"
       app_secret: "8YuPQoUs00l2OlLrMMmXPeXZ0O3kNKyy"
       connection_mode: "websocket"
       allow_from: []  # 允许所有用户
   ```

5. **启动服务**
   ```bash
   ./wukongbot run --config config.yaml
   ```

### Webhook HTTP 模式

如果需要在 Webhook 模式下运行，需要公网 IP 或内网穿透：

1. 配置服务器的公网地址
2. 在飞书开放平台设置 Request URL：`http://你的服务器:12345/feishu/events`
3. 配置文件：
   ```yaml
   channels:
     feishu:
       enabled: true
       connection_mode: "webhook"
       verification_token: "你的验证令牌"
   ```

4. 确保 HTTP gateway 启用：
   ```yaml
   gateway:
     enabled: true
     host: "0.0.0.0"
     port: 12345
   ```

## Bootstrap 文件（可选）

在工作空间中创建这些文件来自定义代理行为：

- `AGENTS.md` - 代理配置
- `SOUL.md` - 个性/角色定义
- `USER.md` - 用户特定指令
- `TOOLS.md` - 工具使用指南
- `IDENTITY.md` - 核心身份配置

## 项目结构

```
wukongbot-go/
├── cmd/wukongbot/          # CLI 入口点
├── internal/
│   ├── agent/            # 核心 Agent 逻辑
│   │   ├── loop.go       # 主处理循环
│   │   ├── context.go    # 上下文构建器
│   │   ├── memory.go     # 内存系统
│   │   ├── skills.go     # 技能加载器
│   │   └── subagent.go   # 子代理管理器
│   ├── bus/              # 消息总线
│   ├── channels/         # 渠道实现
│   │   ├── telegram.go
│   │   ├── whatsapp.go
│   │   └── feishu.go
│   ├── config/           # 配置
│   ├── cron/             # 定时任务服务
│   ├── providers/        # LLM 提供商
│   │   ├── factory.go
│   │   ├── openai.go
│   │   ├── anthropic.go
│   │   └── openrouter.go
│   ├── session/          # 存储（SQLite/MySQL）
│   └── swagger/          # Swagger/OpenAPI 集成
│       ├── config.go
│       ├── parser.go
│       ├── generator.go
│       ├── client.go
│       └── registry.go
├── tools/                # 工具实现
│   ├── filesystem.go
│   ├── shell.go
│   ├── web.go
│   ├── message.go
│   ├── spawn.go
│   ├── cron.go
│   └── image.go
└── skills/               # 内置技能
```

## 开发

### 运行测试

```bash
go test ./...
```

### 添加新渠道

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

### 添加新提供商

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

## Docker 部署

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

## 许可证

MIT License

## 贡献

1. Fork 本仓库
2. 创建你的功能分支
3. 提交你的修改
4. 推送到分支

## TODO

- [ ] 实现支持浏览器操作
- [ ] 自我完善修复bug
- [ ] 自我升级热更新
- [ ] 会话压缩
- [ ] Skills智能管理
5. 创建 Pull Request