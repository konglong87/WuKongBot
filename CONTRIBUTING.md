# Contributing to WuKongBot

感谢您对 WuKongBot 的关注！我们欢迎任何形式的贡献，包括但不限于：

- 报告 Bug
- 提出新功能建议
- 提交代码改进
- 完善文档
- 分享使用经验

## 开发环境设置

### 前置要求

- Go 1.21 或更高版本
- Git

### 克隆仓库

```bash
git clone https://github.com/konglong87/wukongbot.git
cd wukongbot
```

### 安装依赖

```bash
go mod download
```

### 运行项目

```bash
# 复制配置文件
cp config.example.yaml config.yaml

# 根据需要修改 config.yaml 中的配置
vim config.yaml

# 运行项目
go run cmd/wukong/main.go
```

## 开发指南

### 代码规范

- 遵循 Go 语言官方代码规范 [Effective Go](https://golang.org/doc/effective_go)
- 使用 `gofmt` 格式化代码
- 为公共函数和类型添加文档注释
- 保持函数简洁，单一职责原则

### 提交规范

提交信息应该清晰描述所做的更改：

```
<type>: <subject>

<body>

<footer>
```

类型(type)可以是：
- feat: 新功能
- fix: 修复 Bug
- docs: 文档更新
- style: 代码格式调整（不影响代码运行）
- refactor: 重构（既不是新增功能，也不是修复 Bug）
- perf: 性能优化
- test: 测试相关
- chore: 构建过程或辅助工具的变动

示例：
```
feat: 添加 GitHub 集成支持

- 实现 GitHub issue 查询功能
- 添加 PR 状态监控
- 更新相关文档

Closes #123
```

### Pull Request 流程

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'feat: 添加某个功能'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

### PR 要求

- 清晰描述 PR 的目的和更改内容
- 关联相关的 Issue
- 确保代码通过测试
- 更新相关文档
- 保持代码风格一致

## 报告 Bug

在提交 Bug 报告前，请确保：

1. 搜索现有的 Issue，确认问题未被报告
2. 提供详细的重现步骤
3. 附上相关的日志和配置信息（注意隐藏敏感信息）
4. 说明您的运行环境（操作系统、Go 版本等）

## 提出新功能

在提出新功能建议前，请：

1. 确认该功能符合项目目标
2. 详细描述功能的使用场景
3. 说明该功能如何提升用户体验
4. 如果可能，提供实现思路或示例代码

## 行为准则

- 尊重所有贡献者
- 接受建设性批评
- 专注于对社区最有利的事情
- 对其他社区成员表示同理心

## 获取帮助

如果您有任何问题，可以通过以下方式获取帮助：

- 提交 Issue
- 加入讨论组
- 查看项目文档

再次感谢您的贡献！
