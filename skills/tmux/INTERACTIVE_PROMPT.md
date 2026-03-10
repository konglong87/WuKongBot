# tmux 交互式会话 - Claude Code 使用指南

## ⚠️ 重要：tmux 的两步操作模式

tmux 命令**不会自动返回输出**。所有交互都需要两步：

### 正确方式（两步操作）

**步骤 1：发送命令**
```bash
tmux -S $SOCKET send-keys -t s1 "your command" Enter
```

**步骤 2：等待并捕获输出**
```bash
sleep 0.5
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

### 常见场景示例

#### 1. 启动 Claude Code
```bash
# Step 1: 发送命令
tmux -S $SOCKET send-keys -t s1 "opencode --dangerously-skip-permissions" Enter

# Step 2: 等待并捕获输出
sleep 1
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

#### 2. 响应交互式问题（选择题）
```bash
# Step 1: 发送选择（数字）
tmux -S $SOCKET send-keys -t s1 "1" Enter

# Step 2: 捕获响应
sleep 0.5
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

#### 3. 发送用户输入
```bash
# Step 1: 发送文本
tmux -S $SOCKET send-keys -t s1 "user input text" Enter

# Step 2: 捕获响应
sleep 0.5
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

#### 4. 仅按 Enter 键
```bash
# Step 1: 发送 Enter
tmux -S $SOCKET send-keys -t s1 Enter

# Step 2: 捕获响应
sleep 0.5
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

## 监控和调试

```bash
# 实时附加到会话（手动调试）
tmux -S $SOCKET attach -t s1

# 列出所有会话
tmux -S $SOCKET list-sessions

# 查看当前 pane 内容
tmux -S $SOCKET capture-pane -t s1 -p -J -S -200
```

## 核心原则

1. **永远记住两步操作**：send-keys → sleep → capture-pane
2. **等待时间**：简单命令 0.5s，启动应用 1-2s
3. **捕获历史**：使用 `-S -200` 捕获最近 200 行
4. **输出格式**：使用 `-J` 合并换行，`-p` 直接输出到 stdout