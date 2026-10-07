// Package configs holds process configuration for the Douyin MCP server.
package configs

import (
	"os"
	"strconv"
	"strings"
)

// Config is the resolved server configuration.
type Config struct {
	// CookieString is the Douyin Cookie header value (empty => login flow).
	CookieString string
	// CookiesFile is the path the cookie string is read from / written to.
	CookiesFile string
	// Credentials captured from the browser (bd-ticket-guard + dtrait).
	Ticket        string
	TsSign        string
	ClientCert    string
	PrivateKey    string
	DtraitBlob    string
	SessionDtrait string
	// Transport.
	Proxy string
	// Server.
	Port      string
	AuthToken string
	Headless  bool
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// Load resolves configuration from the environment. The cookie string is
// preferred from DOUYIN_COOKIES/DY_COOKIES and otherwise read from the cookie
// file by the caller (cookies package).
func Load() *Config {
	port := firstEnv("DOUYIN_PORT")
	if port == "" {
		port = ":18080"
	}
	return &Config{
		CookieString:  firstEnv("DOUYIN_COOKIES", "DY_COOKIES"),
		CookiesFile:   firstEnv("DOUYIN_COOKIES_FILE"),
		Ticket:        firstEnv("DOUYIN_TICKET", "DY_TICKET"),
		TsSign:        firstEnv("DOUYIN_TS_SIGN", "DY_TS_SIGN"),
		ClientCert:    firstEnv("DOUYIN_CLIENT_CERT", "DY_CLIENT_CERT"),
		PrivateKey:    firstEnv("DOUYIN_PRIVATE_KEY", "DY_PRIVATE_KEY"),
		DtraitBlob:    firstEnv("DOUYIN_DTRAIT_BLOB", "DY_DTRAIT_BLOB"),
		SessionDtrait: firstEnv("DOUYIN_SESSION_DTRAIT", "DY_SESSION_DTRAIT"),
		Proxy:         firstEnv("DOUYIN_PROXY", "DY_PROXY", "HTTPS_PROXY"),
		Port:          port,
		AuthToken:     firstEnv("AUTH_TOKEN", "DOUYIN_AUTH_TOKEN"),
		Headless:      firstEnv("DOUYIN_HEADLESS") != "0",
	}
}

// EnvBool parses a boolean environment variable.
func EnvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
