# Project Guidelines

## 本地开发规范

- 要求每次修改完后，需要帮我格式化 Go 源码文件（`gofmt`）。
- 测试过程中产生的脚本和 build 中间文件，如果没有必要，则删除。
- 所有的 feature 变更，都需要使用分支进行开发。
- 在我未同意之前，你不能推送到远程。
- 我需要：1. 本地 review；2. 远程 PR review。
- 不要过度设计，保持代码的简洁和易读。
- 使用中文注释，一定要简洁明了。专业名词可以用英文。

## 构建与验证

- 统一入口：`go build ./...`、`go vet ./...`、`go test ./...`。
- 需要真实 cookie 的联网测试用环境变量 `DOUYIN_LIVE_TEST=1` 才能跑，默认跳过；CI 只跑 hermetic 单测。
- 主程序与登录工具两个二进制一起构建：`go build .` 与 `go build ./cmd/login`。

## 发版规范

- 发版 = 打语义化 tag `vX.Y.Z` 推上去（触发 Release 与 Docker 镜像），破坏性变更进 major。

## PR Review 重点

- 涉及签名的改动（`douyin/abogus.go`、`xbogus.go`、`secsdk.go`、`bd_ticket.go`、`fpk.go`、`mstoken.go`、`dtrait.go`）必须附上与真实抓包/浏览器实际请求的逐字节对比或 fixture 测试。
- 请求头顺序、`content-length`、query 组等传输层细节容易引发 CDN 400，改动需有回归测试。
- 不在仓库里落任何 cookie / 抓包 / ticket-guard 真实材料。
