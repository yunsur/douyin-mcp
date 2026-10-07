// Command login 是抖音 MCP 的独立登录工具。
//
// 与服务同源：复用 douyin 包里的纯 HTTP 登录流程（扫码 / 手机验证码），
// 登录成功后把 cookie 字符串写入 cookies.txt，供 douyin-mcp 服务启动时读取。
//
// 与 xiaohongshu-mcp 的 cmd/login 保持同样的定位：一个不带服务、只负责登录
// 的小工具，发版时和主程序一起交叉编译分发给用户。
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/yunsur/douyin-mcp/configs"
	"github.com/yunsur/douyin-mcp/cookies"
	"github.com/yunsur/douyin-mcp/douyin"
)

func main() {
	var (
		cookiePath string
		timeoutSec int
		phone      string
		code       string
		proxy      string
		qrOut      string
	)
	flag.StringVar(&cookiePath, "cookies", "", "cookie 文件路径，留空读取 DOUYIN_COOKIES_FILE 或 ./cookies.txt")
	flag.IntVar(&timeoutSec, "timeout", 300, "扫码登录超时时间（秒）")
	flag.StringVar(&phone, "phone", "", "手机号，填写后走短信验证码登录")
	flag.StringVar(&code, "code", "", "短信验证码，配合 -phone 使用；留空则交互输入")
	flag.StringVar(&proxy, "proxy", "", "代理地址，如 http://127.0.0.1:7890，留空读取 DOUYIN_PROXY")
	flag.StringVar(&qrOut, "qr", "login_qrcode.png", "二维码图片保存路径")
	flag.Parse()

	if cookiePath == "" {
		cookiePath = cookies.GetCookiesFilePath()
	}
	if proxy == "" {
		proxy = configs.Load().Proxy
	}
	logrus.Infof("cookie 文件: %s", cookiePath)

	client, err := douyin.NewClient("", douyin.Options{Proxy: proxy})
	if err != nil {
		logrus.Fatalf("初始化客户端失败: %v", err)
	}

	ctx := context.Background()

	var loggedIn *douyin.Client
	if phone != "" {
		loggedIn = loginByPhone(ctx, client, phone, code)
	} else {
		loggedIn = loginByQRCode(ctx, client, timeoutSec, qrOut)
	}

	if err := cookies.SaveCookie(cookiePath, loggedIn.CookieStr()); err != nil {
		logrus.Fatalf("保存 cookie 失败: %v", err)
	}
	logrus.Infof("登录成功，cookie 已保存到 %s", cookiePath)
}

// loginByQRCode 走扫码流程，onQRCode 回调把二维码图片落盘方便用户扫描。
func loginByQRCode(ctx context.Context, client *douyin.Client, timeoutSec int, qrOut string) *douyin.Client {
	logrus.Info("开始扫码登录，请使用抖音 App 扫描二维码...")

	loggedIn, err := client.LoginByQRCode(ctx, timeoutSec, func(data map[string]any) {
		if path, ok := writeQRCodeImage(data, qrOut); ok {
			logrus.Infof("二维码已保存: %s（请打开图片后扫码）", path)
		} else {
			logrus.Infof("二维码数据: %v", data)
		}
	})
	if err != nil {
		logrus.Fatalf("扫码登录失败: %v", err)
	}
	return loggedIn
}

// loginByPhone 走短信验证码流程，未提供 -code 时从标准输入读取。
func loginByPhone(ctx context.Context, client *douyin.Client, phone, code string) *douyin.Client {
	if code == "" {
		if _, err := client.SendPhoneCode(ctx, phone); err != nil {
			logrus.Fatalf("发送短信验证码失败: %v", err)
		}
		logrus.Info("验证码已发送，请输入短信验证码：")
		if _, err := fmt.Scanln(&code); err != nil {
			logrus.Fatalf("读取验证码失败: %v", err)
		}
	}

	loggedIn, err := client.LoginByPhone(ctx, phone, code)
	if err != nil {
		logrus.Fatalf("手机号登录失败: %v", err)
	}
	return loggedIn
}

// writeQRCodeImage 在 get_qrcode 返回的 data 里找出 base64 PNG 字段并落盘。
// 字段名随版本变化，所以不写死 key，按 PNG magic 识别。
func writeQRCodeImage(data map[string]any, path string) (string, bool) {
	for _, v := range data {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
		if err != nil || len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
			continue
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			continue
		}
		return path, true
	}
	return "", false
}
