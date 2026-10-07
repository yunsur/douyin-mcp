# douyin-mcp

[![Go Version](https://img.shields.io/badge/Go-1.26-blue?style=flat-square&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](./LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey?style=flat-square)]()
[![Docker Pulls](https://img.shields.io/docker/pulls/yunsur/douyin-mcp?style=flat-square&logo=docker)](https://hub.docker.com/r/yunsur/douyin-mcp)

MCP for Douyin / douyin.com. Give your AI assistant direct access to Douyin data: **read, interact, direct messages, and creator publishing**.

- **HTTP API**: `/api/v1/*`, Gin with a unified JSON envelope, easy to integrate from any language/frontend
- **MCP**: `/mcp` (Streamable HTTP, official `modelcontextprotocol/go-sdk`)
- **Built-in signatures**: `a_bogus`, `X-Bogus`, `x-secsdk-web-signature`, bd-ticket-guard (ECDH/HKDF/ECDSA/HMAC), `x-tt-session-dtrait`, `fpk1/fpk2`, mssdk `msToken` — no external signing service
- **No browser dependency**: the HTTP transport mimics Chrome fingerprinting; HTTP / WebSocket / uploads all run in-process. No Chromium required.

> 中文文档见 [README.md](./README.md).

### 📖 Resources

- **HTTP API docs**: [docs/API.md](./docs/API.md)
- **API coverage & calibration**: [docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md)
- **Contributing**: [CONTRIBUTING.md](./CONTRIBUTING.md)
- **Docker**: [docker/README.md](./docker/README.md)
- **Windows guide**: [docs/windows_guide.md](./docs/windows_guide.md)

## Overview

**Main features**

1. **Login & status** — QR login (Base64 image + token polling) and phone + SMS login; cookie is written to `cookies.txt`. Tools: `check_login_status`, `get_login_qrcode`, `check_login_qrcode`, `send_phone_code`, `login_by_phone`, `delete_cookies`.
2. **Users, posts & comments** — profiles, posts (single page / all), post detail, top-level comments and replies. Tools: `get_user_info`, `get_user_posts`, `get_user_all_posts`, `get_video_detail`, `get_video_comments`, `get_all_video_comments`, `get_sub_comments`.
3. **Search** — videos / users / lives / suggestions / hot board / challenges. Tools: `search_videos`, `search_users`, `search_lives`, `search_suggest`, `get_hot_search_board`, `search_challenges`.
4. **Relations, notifications & collections** — followers/following, notifications, favorites, watch history, watch later, appointments, collections, home feed.
5. **Interactions** — like, comment, collect / move / uncollect. Tools: `digg_video`, `post_video_comment`, `collect_video`, `move_collect_video`, `remove_collect_video`.
6. **Live rooms** — info, real-time danmaku/gift/member/like/room_stats/PK events, send danmaku, like, rankings, commerce products.
7. **Direct messages (group & private)** — conversation list, chat history, participants, mark read, pin/mute, leave, rename, kick, recall/delete; send text/image/video/voice/file/sticker/card/share; WebSocket reception.
8. **Creator publishing** — publish image posts and videos. Requires bd-ticket-guard material and `DOUYIN_DTRAIT_BLOB`. Tools: `publish_content`, `publish_with_video`.

> **133 MCP tools** in total. The authoritative list is returned by the server's `ListTools`.

## 1. Tutorial

### 1.1. Quick start

**Option A: download prebuilt binaries**

Download from [GitHub Releases](../../releases):

- Server: `douyin-mcp-{darwin-arm64,darwin-amd64,linux-amd64,linux-arm64,windows-amd64.exe}`
- Login tool: `douyin-login-{darwin-arm64,darwin-amd64,linux-amd64,linux-arm64,windows-amd64.exe}`

```bash
# 1. run the login tool first (writes cookies.txt)
chmod +x douyin-login-darwin-arm64
./douyin-login-darwin-arm64

# 2. start the MCP server
chmod +x douyin-mcp-darwin-arm64
./douyin-mcp-darwin-arm64
```

> No headless browser is downloaded — the binaries are small and start instantly. Node.js is only needed (optionally) when Douyin serves the `__ac_signature` challenge page.

**Option B: build from source**

Requires Go 1.26+.

```bash
go build -o douyin-mcp .
go build -o douyin-login ./cmd/login

# or run directly
go run .
go run ./cmd/login
```

**Option C: Docker (easiest)**

```bash
docker pull yunsur/douyin-mcp
cd docker && docker compose up -d && docker compose logs -f
```

See [docker/README.md](./docker/README.md) for details.

### 1.2. Login

```bash
# QR login (saves login_qrcode.png)
./douyin-login-darwin-arm64

# Phone + SMS
./douyin-login-darwin-arm64 -phone 138xxxxxxxx
```

### 1.3. Start the MCP server

```bash
./douyin-mcp-darwin-arm64 -port :18080 -token your-secret-token
```

Optional proxy: `DOUYIN_PROXY=http://user:pass@proxy:port` (HTTP/HTTPS/SOCKS5).
Optional auth: set `AUTH_TOKEN` (or `-token`, which wins); clients must then send `Authorization: Bearer <token>`.

See [docs/API.md](./docs/API.md#环境变量) for the full environment variable list.

### 1.4. Verify

```bash
npx @modelcontextprotocol/inspector
```

Connect to `http://localhost:18080/mcp`, then click `List Tools`.

## 2. MCP clients

```bash
claude mcp add --transport http douyin-mcp http://localhost:18080/mcp
```

Generic HTTP MCP config:

```json
{
  "name": "douyin-mcp",
  "url": "http://localhost:18080/mcp",
  "type": "http"
}
```

Sample configs: [.cursor/mcp.json](./.cursor/mcp.json), [.vscode/mcp.json](./.vscode/mcp.json).

### FAQ

- **Not logged in?** Run the login tool; make sure `cookies.txt` is in the working directory or set `DOUYIN_COOKIES_FILE`.
- **Write APIs report missing ticket-guard material?** Set `DOUYIN_TICKET` / `DOUYIN_TS_SIGN` / `DOUYIN_CLIENT_CERT` / `DOUYIN_PRIVATE_KEY` (and `DOUYIN_DTRAIT_BLOB` for publishing). Read-only APIs are unaffected.
- **`400` / signature errors?** Make sure `DY_HTTP_PROFILE` and `DY_FP_*` match the browser the cookie came from.
- **Challenge page?** The server solves `__ac_signature` locally with Node (`DY_NODE` → PATH → `mise which node`) and retries once; or inject `DY_AC_SIGNATURE` / `DY_AC_NONCE`.

## 3. Technical notes

Signatures: `a_bogus`, `X-Bogus`, `x-secsdk-web-signature`, `SM3`, bd-ticket-guard, `x-tt-session-dtrait`, mssdk `msToken`, `fpk1`, `fpk2` are all generated in-process and verified against fixtures. Only `__ac_signature` runs the page's acrawler VMP via Node (optional).

> The bundled browser-fingerprint profiles under `douyin/profiles/` ship **neutralized** (generic Intel UHD 630, 1920x1080, 8 cores / 8 GB) so the defaults do not single out one machine. Override individual fields with `DY_FP_*`, or replace the profile files, to match a specific browser.

Live events: `LiveEvent.Kind` ∈ `chat` / `gift` / `member` / `like` / `social` / `room_stats` / `pk`; messages are de-duplicated by `(method, msgId)` (512-entry LRU).

See [docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md) for API coverage and calibration details.

## 4. Known limitations

- Creator publishing requires bd-ticket-guard material; publishing additionally requires `DOUYIN_DTRAIT_BLOB`.
- The creator **AI cover** pipeline is not ported (depends on OpenCV/WebCodecs); publishing uses VOD snapshots or an explicit cover.
- Group/message admin endpoints (rename, kick, recall, delete) are implemented but were **not executed against real groups/conversations**.
- Group join/approval, blocking, and message forward/quote are not implemented.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

```bash
go build ./... && go vet ./... && go test ./...
```

## Disclaimer

This project is for learning and research only. Please comply with Douyin's terms of service and applicable laws, and respect creators' copyright. APIs and risk-control policies may change at any time; use at your own risk.

## License

[MIT](./LICENSE) © yunsur
