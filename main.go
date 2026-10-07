package main

import (
	"flag"

	"github.com/sirupsen/logrus"
	"github.com/yunsur/douyin-mcp/configs"
	"github.com/yunsur/douyin-mcp/cookies"
)

// version is injected at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	var (
		port  string
		token string
	)
	flag.StringVar(&port, "port", "", "监听端口（默认读取 DOUYIN_PORT 或 :18080）")
	flag.StringVar(&token, "token", "", "鉴权 Token，留空则读取 AUTH_TOKEN")
	flag.Parse()

	cfg := configs.Load()
	if port == "" {
		port = cfg.Port
	}
	if token == "" {
		token = cfg.AuthToken
	}

	logrus.Infof("douyin-mcp version: %s", version)

	cookiePath := cookies.GetCookiesFilePath()
	cookieStr := cfg.CookieString
	if cookieStr == "" {
		cookieStr = cookies.LoadCookie(cookiePath)
	}

	service, err := NewDouyinService(cfg, cookiePath, cookieStr)
	if err != nil {
		logrus.Fatalf("初始化 Douyin 服务失败: %v", err)
	}

	appServer := NewAppServer(service, token)
	if err := appServer.Start(port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
