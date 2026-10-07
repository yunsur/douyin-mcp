# Windows 安装指南（避免环境变量问题）

在 Windows 部署过程中如果遇到问题，可以先参考本手册。

本项目**不依赖无头浏览器**；只有 `__ac_signature` 挑战页求解会用到 Node.js（可选）。为避免后续环境变量问题，建议用 Winget 安装 Go 与 Node.JS —— 用 Winget 安装后 Windows 会自动配置好对应的环境变量。

## 打开命令行

1. Windows 搜索框中输入 `CMD`
2. 选择「以管理员身份运行」

## 安装 Go（源码编译才需要）

```bash
winget install GoLang.Go
```

## 安装 Node.JS（可选）

仅在抖音返回 `__ac_signature` 挑战页时才需要。若不需要可跳过。

```bash
winget install OpenJS.NodeJS.LTS
```

## 1. 下载最新构建版本

从 [GitHub Releases](../../releases) 下载：

- **主程序**：`douyin-mcp-windows-amd64.exe`
- **登录工具**：`douyin-login-windows-amd64.exe`

下载完解压到同一个文件夹，在该文件夹中右键打开终端。

## 2. 登录

先运行登录工具（扫码会把二维码图片 `login_qrcode.png` 生成在当前目录；也可用 `-phone` 走短信登录）：

```bash
.\douyin-login-windows-amd64.exe
```

登录成功后 cookie 会写入当前目录的 `cookies.txt`。

## 3. 启动 MCP 服务

```bash
.\douyin-mcp-windows-amd64.exe
```

默认监听 `:18080`。可用参数：

```bash
.\douyin-mcp-windows-amd64.exe -port :18080 -token your-secret-token
```

## 4. 解决 Windows Defender 误报

部分杀软会把未签名的自编译二进制判为可疑。若被拦截，加入排除项：

1. 打开 Windows 安全中心（Windows Security）
2. 点击「病毒和威胁防护」
3. 在「病毒和威胁防护设置」下点击「管理设置」
4. 向下滚动，点击「添加或删除排除项」
5. 点击「添加排除项」→ 选择「文件夹」
6. 选择存放 `douyin-mcp-windows-amd64.exe` 的目录以及程序报错时提示的 `%TEMP%` 子目录
7. 确认添加

## 5. MCP 验证

```bash
npx @modelcontextprotocol/inspector
```

在 Inspector 中连接 `http://localhost:18080/mcp`，点击 `Connect`，再点 `List Tools` 查看全部工具。

## 6. 代理（可选）

```bash
set DOUYIN_PROXY=http://user:pass@proxy:port
.\douyin-mcp-windows-amd64.exe
```

支持 HTTP/HTTPS/SOCKS5 代理。
