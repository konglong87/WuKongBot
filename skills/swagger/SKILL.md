---
name: swagger
description: Medical API integration - query and interact with the iHealth medical platform via Swagger/OpenAPI defined endpoints. Only use this skill when you need to interact with EXTERNAL web APIs.
always: false
---

# 🏥 Medical API swagger

**🚨 CRITICAL WARNING:**
1. **ONLY** use the built-in `api` tool for all API operations
2. **DO NOT** use `exec` tool with `curl` commands
3. **DO NOT** call HTTP endpoints directly
4. **ALWAYS** use `api(action="...")` format

## Correct Usage ✅

```
api(action="list")                                    # List all available APIs
api(action="search", keyword="待办")                  # Search for reminder APIs
api(action="call", method="POST", path="/reminder/getReminderEventList")  # Call an API
```

## Wrong Usage ❌

```
curl https://api.ihuyu.top/reminder/getReminderEventList    ← WRONG!
exec curl ...                                             ← WRONG!
POST /reminder/getReminderEventList                        ← WRONG!
```

---

## Core Actions

### 1. List Available APIs

```
api(action="list")
api(action="list", tag="Reminder")
```

**展示所有可用的 API 端点，支持按 tag 过滤**

---

### 2. Search APIs

```
api(action="search", keyword="待办")
api(action="search", keyword="患者", tag="User")
api(action="search", keyword="登录", tag="Base")
```

**通过关键字搜索 API（路径、描述、标签）**

---

### 3. Call API

#### 无参数调用
```
api(action="call", method="GET", path="/notification/getNotification")
```

#### 带 body 参数
```
api(action="call", method="POST", path="/base/captchaLogin",
    body={"phone": "13800138000", "captcha": "123456"})
```

#### 带 query 参数
```
api(action="call", method="POST", path="/user/getPatientsList",
    query={"page": 1, "size": 10})
```

#### 带 path 参数
```
api(action="call", method="GET", path="/user/getUserInfo",
    path={"userId": "123"})
```

---

## Common API Tags

| Tag | Description | Examples |
|-----|-------------|----------|
| `Reminder` | 待办事项 | `/reminder/getReminderEventList`, `/reminder/getReminderEventCntByCount` |
| `Base` | 基础认证 | `/base/login`, `/base/captcha`, `/base/captchaLogin` |
| `User` | 用户/患者管理 | `/user/getPatientFuzzyQuery`, `/user/getPatientsList` |
| `medicalReport` | 医疗记录 | `/medicalReport/getPrescriptions`, `/medicalReport/getPatientRecord` |
| `outpatient` | 门诊预约 | `/outpatient/outpatientAppointment`, `/outpatient/getPatientAppointment` |
| `Notification` | 通知 | `/notification/getNotification` |
| `Article` | 文章 | `/article/getArticleList`, `/article/getArticleDetail` |

---

## Common Usage Scenarios

### 查询待办事项

```
api(action="call", method="POST", path="/reminder/getReminderEventList")
```

### 用户登录

```
api(action="call", method="POST", path="/base/captchaLogin",
    body={"phone": "13800138000", "captcha": "123456"})
```

### 查询患者

```
api(action="call", method="POST", path="/user/getPatientFuzzyQuery",
    body={"keyword": "张三"})
```

### 查询医疗记录

```
api(action="call", method="POST", path="/medicalReport/getPrescriptions")
```

### 获取通知

```
api(action="call", method="POST", path="/notification/getNotification")
```

---

## Complete Dialogue Examples

**用户：** "当前待办有哪些？"
**你执行：**
```
api(action="call", method="POST", path="/reminder/getReminderEventList")
```

---

**用户：** "查询患者张三的信息"
**你执行：**
```
api(action="call", method="POST", path="/user/getPatientFuzzyQuery",
    body={"keyword": "张三"})
```

---

**用户：** "有哪些医疗相关的 API？"
**你执行：**
```
api(action="search", keyword="医疗")
```

---

**用户：** "获取我的通知"
**你执行：**
```
api(action="call", method="POST", path="/notification/getNotification")
```

---

## Parameters Reference

| Parameter | Type | Required for | Description |
|-----------|------|--------------|-------------|
| `action` | string | ✅ All | `list`, `call`, `search` |
| `method` | string | call | HTTP method: GET, POST, PUT, DELETE |
| `path` | string | call | API path: e.g., `/reminder/getReminderEventList` |
| `path_params` | object | call (optional) | Path parameters: `{"userId": "123"}` |
| `query_params` | object | call (optional) | Query parameters: `{"page": 1, "size": 10}` |
| `body` | object | call (optional) | Request body: `{"phone": "...", "captcha": "..."}` |
| `keyword` | string | search | Search keyword |
| `tag` | string | list, search (optional) | Filter by API tag |

---

## Important Rules

⚠️ **只使用 `api` 工具，不要用其他任何方式调用 API！**

- ❌ 不要用 `curl` 或 HTTP 直接调用
- ❌ 不要用 `exec` 执行命令
- ✅ 只用 `api(action="...", ...)`
