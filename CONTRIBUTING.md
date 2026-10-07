# 贡献指南 | Contributing Guide

感谢你对本项目的关注！为了保证代码质量和 Review 效率，请在提交 PR 前仔细阅读以下规范。

Thank you for your interest! Please read this guide carefully before submitting a PR.

---

## 基本流程 | Basic Workflow

1. Fork 本仓库并创建功能分支
2. 在本地完成开发和测试
3. 提交 PR 并填写清晰的描述

---

## PR 提交规范 | PR Requirements

### 1. 一个 PR 只做一件事 | One PR, One Feature

每个 PR 只包含 **一个功能或一个修复**。多个功能请拆分为多个 PR。

Each PR should contain **only one feature or one fix**. Split multiple features into separate PRs.

### 2. 必须经过验证 | Must Be Verified

**即使代码是 AI 生成的，也必须在本地运行并验证功能正确。** 未经验证的 PR 将直接关闭。

**Even if the code is AI-generated, you must run and verify it locally.** Unverified PRs will be closed.

### 3. 提供演示截图/视频 | Provide Demo

PR 中请附上功能演示的 **截图或录屏**，让 Reviewer 快速理解改动效果。

Please attach **screenshots or screen recordings** to demonstrate the feature.

> **隐私提醒：演示中务必对自己的账号信息（cookie / uid / 昵称）进行打码处理！**
>
> **Privacy: Always blur/mask your account info in demos!**

### 4. 不要提交凭据与抓包产物 | No Credentials or Scratch Captures

- **严禁**提交 `cookies.txt`、ticket-guard 材料、`.har` 抓包文件、`conversations_cache.json` 等敏感/运行态产物。
- 新增接口请附上请求/响应结构说明（脱敏后），不要贴真实 cookie。

### 5. 代码规范 | Code Style

- Go 代码需要格式化（`gofmt`）
- 注释使用中文，专业术语可用英文
- 不要过度设计，保持简洁

### 6. 测试规范 | Testing

```bash
go test ./...                 # 全部单元测试（无网络）
go test -race -count=1 ./...  # CI 以这个为准
```

- **必须 hermetic**：单元测试不得联网、不得 `os/exec`、不得启动浏览器，必须离线可复现。
- 需要真实 cookie 的联网测试单独放在 `*_live_test.go`，用 `DOUYIN_LIVE_TEST=1` 门控，默认 `Skip`。
- 打抖音接口的测试通过 `douyin.Transport` 注入桩传输（见 `douyin/stub_transport_test.go` 与根目录 `harness_test.go`），断言**请求形状**（method / host / path / 关键参数 / body）与**响应处理**（成功解码、业务错误码映射、非 JSON、cookie 吸收），而不是只断言“没报错”。
- 不要写重复实现细节的测试（字段赋值、转发、常量回抄），测试要能捕获真实的消费端可见缺陷。

---

## 提交 Checklist | PR Checklist

提交前请确认：

- [ ] 代码已在本地运行并验证通过（`go build ./...`、`go vet ./...`、`go test ./...`）
- [ ] 一个 PR 仅包含一个功能/修复
- [ ] 附上演示截图或录屏（账号信息已打码）
- [ ] 未提交任何 cookie / 抓包 / 运行态文件
- [ ] 代码已格式化，注释清晰

---

感谢你的贡献！🎉 | Thanks for contributing!
