# douyin-mcp

[![Go Version](https://img.shields.io/badge/Go-1.26-blue?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](./LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey?style=flat-square)]()
[![Docker Pulls](https://img.shields.io/docker/pulls/yunsur/douyin-mcp?style=flat-square&logo=docker)](https://hub.docker.com/r/yunsur/douyin-mcp)

MCP for 抖音 / douyin.com。让你的 AI 助手直接访问抖音数据：**读取、互动、私信收发、创作者发布**。

- **HTTP API**：`/api/v1/*`，Gin + 统一 JSON 信封，方便任意语言 / 前端接入
- **MCP**：`/mcp`（Streamable HTTP，官方 `modelcontextprotocol/go-sdk`）
- **内置签名**：`a_bogus`、`X-Bogus`、`x-secsdk-web-signature`、bd-ticket-guard（ECDH/HKDF/ECDSA/HMAC）、`x-tt-session-dtrait`、`fpk1/fpk2`、mssdk `msToken`，不依赖任何外部签名服务
- **无浏览器依赖**：HTTP 传输层模拟 Chrome 指纹，HTTP / WebSocket / 上传全部在本进程内完成，不需要 Chromium

> [!IMPORTANT]
> #### 🔥 方案 A：Agent Skills 深度集成（推荐给开发者）
> - **[douyin-mcp-skills](https://github.com/yunsur/douyin-mcp-skills)**：基于本项目的 Agent Skills 集合，12 个 skill 覆盖全部 MCP 工具（登录 / 搜索 / 浏览 / 互动 / 用户 / 收藏 / 通知 / 发布 / 私信 / 直播 / 内容策划 + 一键部署），适配 OpenClaw / Claude Code / omp —— 适用于已部署完本项目的用户。
> - 装好后直接说「搜一下某某」「看下 Mz5 群未读」「监听这个直播间的弹幕」即可，无需手写工具调用。

### 📖 相关资源

- **Agent Skills**：[douyin-mcp-skills](https://github.com/yunsur/douyin-mcp-skills)（OpenClaw / Claude Code / omp 通用）
- **HTTP API 文档**：[docs/API.md](./docs/API.md)
- **接口覆盖与校准**：[docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md)
- **贡献指南**：[CONTRIBUTING.md](./CONTRIBUTING.md)
- **Docker 部署**：[docker/README.md](./docker/README.md)
- **Windows 安装**：[docs/windows_guide.md](./docs/windows_guide.md)

## 项目简介

**主要功能**

<details>
<summary><b>1. 登录和检查登录状态</b></summary>

支持扫码登录（返回 Base64 二维码 + token 轮询）与手机号 + 短信验证码登录，登录成功后 cookie 自动写入本地 `cookies.txt`。

- MCP 工具：`check_login_status`、`get_login_qrcode`、`check_login_qrcode`、`send_phone_code`、`login_by_phone`、`delete_cookies`

</details>

<details>
<summary><b>2. 用户、作品与评论</b></summary>

用户资料、用户作品（单页 / 全部）、作品详情、一级评论与二级回复。评论一律返回抖音原始结构。

- MCP 工具：`get_user_info`、`get_user_posts`、`get_user_all_posts`、`get_video_detail`、`get_video_comments`、`get_all_video_comments`、`get_sub_comments`

</details>

<details>
<summary><b>3. 搜索</b></summary>

视频 / 用户 / 直播 / 联想建议 / 热搜榜 / 话题挑战，覆盖当前 PC 版全部搜索面。

- MCP 工具：`search_videos`、`search_users`、`search_lives`、`search_suggest`、`get_hot_search_board`、`search_challenges`

</details>

<details>
<summary><b>4. 关系、通知与收藏</b></summary>

粉丝 / 关注列表、消息通知（分区、未读数、详情、删除）、喜欢 / 观看历史 / 稍后再看 / 我的预约 / 收藏夹 / 推荐流。

- MCP 工具：`get_user_followers`、`get_user_following`、`get_notice_count`、`get_notices`、`get_notice_detail`、`delete_notice`、`get_user_favorites`、`get_watch_history`、`get_watch_later`、`get_my_appointments`、`get_collect_list`、`get_homefeed`

</details>

<details>
<summary><b>5. 互动操作</b></summary>

点赞 / 取消点赞、发表 / 回复评论、收藏 / 移动收藏 / 取消收藏。

- MCP 工具：`digg_video`、`post_video_comment`、`collect_video`、`move_collect_video`、`remove_collect_video`

</details>

<details>
<summary><b>6. 直播间监听与互动</b></summary>

直播间信息、弹幕 / 礼物 / 进场 / 点赞 / 房间热度 / PK 事件实时监听、发弹幕、点赞、各类榜单、带货商品。

- MCP 工具：`get_live_info`、`start_live_listen`、`stop_live_listen`、`get_live_events`、`send_live_comment`、`like_live_room` 及各榜单工具

</details>

<details>
<summary><b>7. 私信收发（群聊 / 私聊）</b></summary>

会话列表（含群名与未读数）、聊天历史（游标翻页 / 只取未读）、群成员、标记已读、置顶免打扰、退群、改群名、踢人、撤回 / 删除消息；发送文本 / 图片 / 视频 / 语音 / 文件 / 表情包 / 卡片 / 分享；WebSocket 实时接收。

- MCP 工具：`list_conversations`、`get_conversation_history`、`get_conversation_info`、`get_conversation_participants`、`send_dm`、`start_im_listen`、`get_im_messages` 等

</details>

<details>
<summary><b>8. 创作者发布</b></summary>

发布图文与视频到创作者中心。需要配置 bd-ticket-guard 材料，发布另需 `DOUYIN_DTRAIT_BLOB`。

- MCP 工具：`publish_content`、`publish_with_video`

</details>

> 共 **133 个 MCP 工具**，完整清单以服务端 `ListTools` 返回为准。

## 1. 使用教程

### 1.1. 快速开始（推荐）

**方式一：下载预编译二进制文件**

从 [GitHub Releases](../../releases) 下载对应平台的二进制文件：

**主程序（MCP 服务）：**

- **macOS Apple Silicon**: `douyin-mcp-darwin-arm64`
- **macOS Intel**: `douyin-mcp-darwin-amd64`
- **Linux x64**: `douyin-mcp-linux-amd64`
- **Linux ARM64**: `douyin-mcp-linux-arm64`
- **Windows x64**: `douyin-mcp-windows-amd64.exe`

**登录工具：**

- **macOS Apple Silicon**: `douyin-login-darwin-arm64`
- **macOS Intel**: `douyin-login-darwin-amd64`
- **Linux x64**: `douyin-login-linux-amd64`
- **Linux ARM64**: `douyin-login-linux-arm64`
- **Windows x64**: `douyin-login-windows-amd64.exe`

使用步骤：

```bash
# 1. 首先运行登录工具，把 cookie 写入当前目录的 cookies.txt
chmod +x douyin-login-darwin-arm64
./douyin-login-darwin-arm64

# 2. 然后启动 MCP 服务
chmod +x douyin-mcp-darwin-arm64
./douyin-mcp-darwin-arm64
```

> 本项目**不需要**下载无头浏览器，二进制体积小、启动快。仅当抖音返回 `__ac_signature` 挑战页时才需要 Node.js（可选）。

**方式二：源码编译**

<details>
<summary>源码编译安装详情</summary>

依赖 Golang 环境（Go 1.26+），安装方法请参考 [Golang 官方文档](https://go.dev/doc/install)。

```bash
# 配置 GOPROXY 环境变量，以下三选一

# 1. 七牛 CDN
go env -w GOPROXY=https://goproxy.cn,direct

# 2. 阿里云
go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# 3. 官方
go env -w GOPROXY=https://goproxy.io,direct
```

```bash
# 编译主程序与登录工具
go build -o douyin-mcp .
go build -o douyin-login ./cmd/login

# 或直接运行
go run .
go run ./cmd/login
```

</details>

**方式三：使用 Docker 容器（最简单）**

<details>
<summary>Docker 部署详情</summary>

```bash
# 1. 拉取镜像
docker pull yunsur/douyin-mcp

# 2. 使用 Docker Compose 启动
cd docker
docker compose up -d
docker compose logs -f
```

**自己构建镜像（可选）**：

```bash
# 在项目根目录运行
docker build -t yunsur/douyin-mcp .
```

Docker 版本会挂载 `./data` 持久化 cookie 与运行数据，并暴露 `18080` 端口。详细说明请参考：[Docker 部署指南](./docker/README.md)。

</details>

Windows 遇到问题首先看这里：[Windows 安装指南](./docs/windows_guide.md)

### 1.2. 登录

第一次需要手动登录，登录状态会保存到 `cookies.txt`（路径可用 `DOUYIN_COOKIES_FILE` 覆盖）。

**使用二进制文件**：

```bash
# 扫码登录（二维码保存为 login_qrcode.png）
./douyin-login-darwin-arm64

# 手机号 + 短信验证码登录
./douyin-login-darwin-arm64 -phone 138xxxxxxxx
```

**使用源码**：

```bash
go run ./cmd/login
```

常用参数：`-cookies <path>`（cookie 文件路径）、`-timeout <秒>`（扫码超时）、`-proxy <url>`（代理）、`-qr <path>`（二维码图片路径）。

### 1.3. 启动 MCP 服务

```bash
# 使用二进制文件
./douyin-mcp-darwin-arm64

# 或使用源码
go run .

# 指定端口 / 鉴权 token
./douyin-mcp-darwin-arm64 -port :18080 -token your-secret-token
```

**配置代理（可选）**：

```bash
DOUYIN_PROXY=http://user:pass@proxy:port ./douyin-mcp-darwin-arm64
```

支持 HTTP/HTTPS/SOCKS5 代理。

**访问鉴权（可选）**：

默认关闭鉴权。生产环境建议使用 `AUTH_TOKEN` 环境变量配置；非空的启动参数优先于环境变量。

```bash
# 环境变量
AUTH_TOKEN=your-secret-token ./douyin-mcp-darwin-arm64

# 非空启动参数（优先于环境变量）
./douyin-mcp-darwin-arm64 -token=your-secret-token
```

启用鉴权后，所有 MCP 客户端都必须配置请求头 `Authorization: Bearer <token>`：

```json
{
  "mcpServers": {
    "douyin-mcp": {
      "url": "http://localhost:18080/mcp",
      "headers": { "Authorization": "Bearer your-secret-token" }
    }
  }
}
```

完整环境变量列表见 [docs/API.md](./docs/API.md#环境变量)。

### 1.4. 验证 MCP

```bash
npx @modelcontextprotocol/inspector
```

运行后打开提示的链接，配置 MCP inspector，输入 `http://localhost:18080/mcp`，点击 `Connect` 按钮，再点击 `List Tools` 查看所有工具。

### 1.5. 使用 MCP 示例

```
搜索抖音上关于「榴莲」的最新视频，挑播放量最高的 3 条，分别总结前 10 条评论的观点。
```

```
获取用户（https://www.douyin.com/user/xxxx）的全部作品，列成表格。
```

```
监听直播间 123456 的弹幕，收到带「福袋」的消息就通知我。
```

## 2. MCP 客户端接入

本服务支持标准的 Model Context Protocol (MCP)，可以接入各种支持 MCP 的 AI 客户端。

### 2.1. 快速开始

```bash
# 启动服务
go run .
```

服务将运行在：`http://localhost:18080/mcp`

**验证服务状态**：

```bash
curl -X POST http://localhost:18080/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}'
```

**Claude Code CLI 接入**：

```bash
# 添加 HTTP MCP 服务器
claude mcp add --transport http douyin-mcp http://localhost:18080/mcp

# 检查是否添加成功（确保 MCP 已经启动）
claude mcp list
```

### 2.2. 支持的客户端

<details>
<summary><b>Claude Code CLI</b></summary>

```bash
claude mcp add --transport http douyin-mcp http://localhost:18080/mcp
claude mcp list
```

</details>

<details>
<summary><b>Cursor</b></summary>

项目已在 [.cursor/mcp.json](./.cursor/mcp.json) 提供示例配置，位于项目根目录时可直接使用。

</details>

<details>
<summary><b>VS Code / Cline</b></summary>

在设置中按 HTTP MCP 添加：

```json
{
  "name": "douyin-mcp",
  "url": "http://localhost:18080/mcp",
  "type": "http"
}
```

VS Code 的示例配置见 [.vscode/mcp.json](./.vscode/mcp.json)。

</details>

### 2.3. 可用 MCP 工具

共 **133 个工具**，覆盖登录、用户与作品、评论、搜索、关系与通知、互动、直播监听与互动、
私信收发、创作者发布。接入后点击 `List Tools` 查看完整清单与参数说明。常用工具：

- `check_login_status` - 检查登录状态（无参数）
- `get_login_qrcode` - 获取登录二维码（返回 Base64 图片和 token）
- `search_videos` - 搜索视频（`keyword`，可选 `count` / `sort_type` / `publish_time`）
- `get_user_info` / `get_user_posts` - 用户资料 / 用户作品
- `get_video_detail` / `get_all_video_comments` - 作品详情 / 全部评论
- `digg_video` / `post_video_comment` / `collect_video` - 点赞 / 评论 / 收藏
- `get_live_info` / `start_live_listen` / `get_live_events` - 直播间信息 / 开始监听 / 拉取事件
- `list_conversations` / `get_conversation_history` / `send_dm` - 私信会话 / 历史 / 发送
- `publish_content` / `publish_with_video` - 发布图文 / 视频到创作者中心

### 2.4. 使用示例

```
使用 douyin-mcp 搜索「露营」相关视频，按最新发布排序，取前 20 条，
对每条列出作者、点赞数、发布时间。
```

```
用 douyin-mcp 读取我关注列表里的用户，统计每个人最近 10 条作品的点赞中位数。
```

### 2.5. 常见问题解答

**Q:** 检查登录状态一直显示未登录？
**A:** 先运行登录工具（`douyin-login` 或 `go run ./cmd/login`）完成扫码/短信登录；确认 `cookies.txt` 与启动目录一致，或通过 `DOUYIN_COOKIES_FILE` 指定绝对路径。

**Q:** 点赞 / 评论 / 发布返回「缺少 ticket-guard 材料」？
**A:** 写接口需要 bd-ticket-guard 材料，请配置 `DOUYIN_TICKET` / `DOUYIN_TS_SIGN` / `DOUYIN_CLIENT_CERT` / `DOUYIN_PRIVATE_KEY`；创作者发布另需 `DOUYIN_DTRAIT_BLOB`。只读接口不受影响。

**Q:** 请求报 `400` 或签名错误？
**A:** 确认 `DY_HTTP_PROFILE`（TLS 指纹档位）与 `DY_FP_*`（指纹档案覆盖）与 cookie 所属浏览器一致；必要的公共 cookie（如 `s_v_web_id`）由客户端自动生成。

**Q:** 抖音返回挑战页（`__ac_signature`）怎么办？
**A:** 服务会先本地用 Node 求解并自动重试一次，Node 解析顺序为 `DY_NODE` → PATH → `mise which node`；也可用 `DY_AC_SIGNATURE` / `DY_AC_NONCE` 注入。三者都不可用时回退为明确报错。

**Q:** MCP 连接 `http://localhost:18080/mcp` 无法连接？
**A:** Docker 环境下注意端口映射与容器内 IP；本机确认服务已启动（`curl localhost:18080/health`）。

## 3. 技术说明

### 签名支持情况

| 签名 | 说明 | 验证 |
| --- | --- | --- |
| `a_bogus` | 内置生成 | 与 5 组 fixture **逐字节一致** |
| `X-Bogus` | 内置生成，用于直播间 WebSocket 握手 | 握手 query 与浏览器逐字节一致 |
| `x-secsdk-web-signature` | 内置生成 | 2 组 fixture + 幂等性测试 |
| `SM3` | 内置摘要算法，`a_bogus` 依赖它 | 标准向量 |
| bd-ticket-guard（ECDH+HKDF / HMAC / ECDSA） | 内置，写接口需要 | 配置 `DOUYIN_TICKET/TS_SIGN/CLIENT_CERT/PRIVATE_KEY` 后写接口实测可用 |
| `x-tt-session-dtrait` | 内置，创作者发布需要 | 配置 `DOUYIN_DTRAIT_BLOB`；发布路径强制校验 |
| mssdk `msToken` | 内置后台换取，失败回退随机值 | 实测换取 **164 字符**真实 token（随机回退为 107） |
| `fpk2` | 内置：MD5(UA) | 单测 |
| `fpk1` | 内置：FingerprintJS 组件串 → murmur3-x64-128 摘要 → AES-256-CBC + OpenSSL `EVP_BytesToKey` | 算法与浏览器一致；摘要由内置档案派生（档案已中性化），定盐结果逐字节稳定 |
| `__ac_signature` | 挑战页命中时用 Node 求解页面 acrawler VMP 并自动重试一次；也可用 `DY_AC_SIGNATURE` / `DY_AC_NONCE` 注入 | 端到端实测产出 47 字符签名 |

除挑战页求解需要 Node（可选）外，其余签名都在本进程内完成，不需要任何外部签名服务。

> 内置的浏览器指纹档案（`douyin/profiles/`）**已中性化**：通用集显（Intel UHD 630）、1920x1080、8 核 / 8GB，避免默认值暴露某一台具体机器。
> 若要与某个真实浏览器完全一致，用 `DY_FP_*` 覆盖（UA / 屏幕 / 核数 / 内存 / WebGL 等），或按需替换档案文件。

### 直播间监听事件

`LiveEvent.Kind` ∈ `chat` / `gift` / `member` / `like` / `social` / `room_stats` / `pk`，
字段名与抖音 Web 端一致（礼物的 `to_user`、`combo_count`，用户的
`sec_uid`/`nickname`，房间热度的 `display_long`，PK 事件为规范化后的 map）。
消息按 `(method, msgId)` 去重（512 条 LRU），重连回放不会重复推送。

### 接口覆盖与校准

搜索面覆盖、五个主导航 tab 覆盖、个人主页各 tab、通知 / 消息 / 私信协议，以及逐接口抓包校准
结论，见 [docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md)。

## 4. 已知边界

- 创作者发布需要 `DOUYIN_TICKET/TS_SIGN/CLIENT_CERT/PRIVATE_KEY`，发布另需 `DOUYIN_DTRAIT_BLOB`；缺失时返回明确错误。
- 创作者 **AI 封面链**未移植（依赖 OpenCV/WebCodecs），发布使用 VOD 截图或显式封面。
- 私信 / 群聊：会话列表、历史消息、会话详情 / 群成员 / 已读 / 置顶免打扰 / 退群 / 分享均已实现；群管理 / 消息管理（改群名、踢人、撤回、删除）已实现，但**未在真实群 / 会话上执行**（避免改动别人的群）。
- 仍未做的是群申请 / 审核、拉黑、`block_members` 等运营类接口，以及消息转发 / 引用。
- `__ac_signature`（acrawler VMP）依赖 Node；均不可用时回退为明确报错。`fpk1`/`fpk2` 已实现；登录仍走非严格路径。

## 贡献

欢迎提交 PR！提交前请阅读 [贡献指南](./CONTRIBUTING.md)。

```bash
go build ./... && go vet ./... && go test -race -count=1 ./...
```

- 单元测试**全部 hermetic**（离线可跑）：打抖音接口的用例通过 `douyin.Transport` 注入桩传输，只断言请求形状与响应处理；`configs` / `cookies` / `cmd/login` 也有独立用例。
- 需要真实 cookie 的联网测试命名为 `TestLive*` 且由 `DOUYIN_LIVE_TEST=1` 门控，默认全部 `Skip`。

## 免责声明

本项目仅供学习与技术研究使用。请遵守抖音平台服务条款及相关法律法规，尊重创作者版权。
接口与风控策略可能随时变化，使用风险自负。

## 许可证

[MIT](./LICENSE) © yunsur
