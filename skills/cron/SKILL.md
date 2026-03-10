---
name: cron
description: Schedule reminders and recurring tasks. Use the built-in 'cron' tool for all operations.
---

# 🔔 Cron 定时任务

**🚨 CRITICAL WARNING:**
1. **DO NOT** use `exec` tool with `curl` commands
2. **DO NOT** call HTTP APIs like `/api/cron`
3. **DO NOT** use system commands like `crontab`
4. **ALWAYS** use the built-in `cron` tool directly

## Correct Usage ✅

```cron(action="list")```
```cron(action="add", message="💧 喝水啦", every_seconds=30, one_time=true)```
```cron(action="remove", job_id="abc123")```

## Wrong Usage ❌

```curl http://localhost/api/cron/list``` ← WRONG!
```exec crontab -l``` ← WRONG!
```POST /api/cron``` ← WRONG!

---

## 核心操作

### 1. 查询所有定时任务
**用户输入：**
- "当前定时任务有哪些"
- "显示所有提醒"
- "查看任务列表"

**立即执行：**
```
cron(action="list")
```

### 2. 创建定时任务

**一次性提醒（延迟后执行一次）：**
```
cron(action="add", message="💧 该喝水啦！", every_seconds=30, one_time=true)
```

**循环提醒（按间隔重复）：**
```
cron(action="add", message="站起来活动一下", every_seconds=300)
```

**定时提醒（使用 cron 表达式）：**
```
cron(action="add", message="早上好！💧", cron_expr="0 8 * * *")
```

### 3. 删除定时任务
```
cron(action="remove", job_id="abc123")
```
注意：先执行 `cron(action="list")` 获取 job_id

---

## 参数说明

| 参数 | 必需 | 说明 |
|------|------|------|
| `action` | ✅ | 操作：`add`, `list`, `remove` |
| `message` | add需要 | 提醒消息内容 |
| `every_seconds` | 否 | 延迟/间隔秒数（30=30秒，300=5分钟，3600=1小时） |
| `cron_expr` | 否 | Cron表达式（"0 8 * * *" = 每天8点） |
| `one_time` | 否 | true=一次性，false=循环 |
| `job_id` | remove需要 | 任务ID（从 list 获取） |

---

## 时间换算表

| 输入 | every_seconds |
|------|---------------|
| 30秒 | 30 |
| 1分钟 | 60 |
| 5分钟 | 300 |
| 10分钟 | 600 |
| 15分钟 | 900 |
| 30分钟 | 1800 |
| 1小时 | 3600 |
| 2小时 | 7200 |
| 6小时 | 21600 |
| 12小时 | 43200 |
| 24小时 | 86400 |

---

## Cron 表达式示例

```
* * * * *      → 每分钟
0 * * * *      → 每小时
0 8 * * *      → 每天 8:00
*/5 * * * *    → 每5分钟
0 */2 * * *    → 每2小时
0 9-17 * * 1-5 → 工作日9-17点（每小时）
0 0 * * 0      → 每周日0点
```

---

## 完整对话示例

**用户：** "当前定时任务有哪些"
**你执行：** `cron(action="list")`

---

**用户：** "30秒后提醒我喝水"
**你执行：**
```
cron(action="add", message="💧 该喝水啦！记得补充水分", every_seconds=30, one_time=true)
```

---

**用户：** "每5分钟提醒我站起来"
**你执行：**
```
cron(action="add", message="⏰ 站起来活动一下", every_seconds=300)
```

---

**用户：** "每天早上8点提醒我喝水"
**你执行：**
```
cron(action="add", message="早上好！该喝水了 💧", cron_expr="0 8 * * *")
```

---

**用户：** "删除任务 abc123"
**你执行：**
```
cron(action="remove", job_id="abc123")
```

---

## 重要提醒

⚠️ **只使用 `cron` 工具，不要用其他任何方式操作定时任务！**

- ❌ 不要用 `curl` 或 HTTP API
- ❌ 不要用 `exec` 执行命令
- ❌ 不要用系统 `crontab` 命令
- ✅ 只用 `cron(action="...")`
