# Docker 使用说明

## 0. 重点注意

写在最前面。

- 本镜像**不含无头浏览器**：抖音链路是纯 Go + uTLS 的 HTTP 实现，无需 Chromium，镜像体积很小。
- 启动后请在容器内完成一次登录，cookie 会写入 `./data/cookies.txt`。请挂载 `./data:/app/data` 持久化登录状态。
- 用本地图片/视频发布时，请把文件放进 `./data/`（或额外挂载的目录），并在调用发布接口时使用**容器内路径**（如 `/app/data/xxx.jpg`），否则一定失败。
- 抖音返回 `__ac_signature` 挑战页时需要 Node（镜像已内置 `nodejs`）；镜像内没有 `mise`，走的是 PATH 上的 `node`。

## 1. 获取 Docker 镜像

### 1.1 从 Docker Hub 拉取（推荐）

```bash
# 拉取最新镜像
docker pull yunsur/douyin-mcp
```

### 1.2 自己构建镜像（可选）

在有项目 Dockerfile 的目录运行：

```bash
docker build -t yunsur/douyin-mcp .
```

`yunsur/douyin-mcp` 为镜像名称。

## 2. 手动 Docker Compose

在 `docker-compose.yml` 文件的同一个目录运行：

```bash
# --- 启动 docker 容器 ---
docker compose up -d

# 查看日志
docker compose logs -f

# 停止
docker compose stop

# 进入容器
docker exec -it douyin-mcp sh

# 手动更新容器
docker compose pull && docker compose up -d
```

## 3. 登录

镜像内置了登录工具 `douyin-login`，在容器内执行一次即可（短信登录示例）：

```bash
docker exec -it douyin-mcp /app/douyin-login -phone 138xxxxxxxx
```

扫码登录会生成 `login_qrcode.png`，把文件 `docker cp` 出来扫描：

```bash
docker exec -it douyin-mcp /app/douyin-login -qr /app/data/login_qrcode.png
docker cp douyin-mcp:/app/data/login_qrcode.png ./login_qrcode.png
```

登录成功后 cookie 写入 `/app/data/cookies.txt`（即宿主机的 `./data/cookies.txt`）。

## 4. 使用 MCP-Inspector 进行连接

**注意 IP 换成你自己的 IP**

```bash
npx @modelcontextprotocol/inspector
```

在 Inspector 里连接 `http://localhost:18080/mcp`。

## 5. 配置代理（可选）

如果需要通过代理访问抖音，可以通过 `DOUYIN_PROXY` 环境变量配置。

### 使用 docker run

```bash
docker run -e DOUYIN_PROXY=http://user:pass@proxy:port yunsur/douyin-mcp
```

### 使用 docker-compose

在 `docker-compose.yml` 的 `environment` 中添加 `DOUYIN_PROXY`：

```yaml
environment:
  - DOUYIN_COOKIES_FILE=/app/data/cookies.txt
  - DOUYIN_PROXY=http://user:pass@proxy:port
```

支持 HTTP/HTTPS/SOCKS5 代理。

## 6. 配置访问鉴权（可选）

不设置或设置为空时，鉴权默认关闭。生产环境建议通过 `AUTH_TOKEN` 环境变量配置访问令牌。

```bash
AUTH_TOKEN=your-secret-token docker compose up -d
```

Compose 通过 `${AUTH_TOKEN:-}` 读取宿主环境变量。启用鉴权后，所有 MCP 客户端请求都必须带上请求头：`Authorization: Bearer <token>`。

## 7. 写接口所需的 ticket-guard 材料（可选）

创作者发布、点赞、评论、私信发送等**写接口**需要 bd-ticket-guard 材料。可在 `environment` 里补充：

```yaml
environment:
  - DOUYIN_TICKET=...
  - DOUYIN_TS_SIGN=...
  - DOUYIN_CLIENT_CERT=...
  - DOUYIN_PRIVATE_KEY=...
  - DOUYIN_DTRAIT_BLOB=...   # 创作者发布必需
```

缺失时写接口会返回明确错误，只读接口不受影响。
