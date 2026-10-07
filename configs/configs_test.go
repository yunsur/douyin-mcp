package configs

import (
	"testing"
)

// allEnvKeys 是 Load/EnvBool 会读取的全部环境变量，测试前统一清空。
var allEnvKeys = []string{
	"DOUYIN_COOKIES", "DY_COOKIES",
	"DOUYIN_COOKIES_FILE",
	"DOUYIN_TICKET", "DY_TICKET",
	"DOUYIN_TS_SIGN", "DY_TS_SIGN",
	"DOUYIN_CLIENT_CERT", "DY_CLIENT_CERT",
	"DOUYIN_PRIVATE_KEY", "DY_PRIVATE_KEY",
	"DOUYIN_DTRAIT_BLOB", "DY_DTRAIT_BLOB",
	"DOUYIN_SESSION_DTRAIT", "DY_SESSION_DTRAIT",
	"DOUYIN_PROXY", "DY_PROXY", "HTTPS_PROXY",
	"DOUYIN_PORT",
	"AUTH_TOKEN", "DOUYIN_AUTH_TOKEN",
	"DOUYIN_HEADLESS",
}

// applyEnv 先把所有已知键置空（等价于未设置），再写入用例提供的值。
func applyEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range allEnvKeys {
		t.Setenv(k, "")
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
}

// defaultConfig 是所有变量都未设置时的期望值。
func defaultConfig() Config {
	return Config{Port: ":18080", Headless: true}
}

func TestLoadDefaults(t *testing.T) {
	applyEnv(t, nil)
	got := Load()
	if want := defaultConfig(); *got != want {
		t.Fatalf("Load() = %+v, want %+v", *got, want)
	}
}

func TestLoadPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "全部主键",
			env: map[string]string{
				"DOUYIN_COOKIES":        "ck",
				"DOUYIN_COOKIES_FILE":   "/tmp/c.txt",
				"DOUYIN_TICKET":         "tk",
				"DOUYIN_TS_SIGN":        "ts",
				"DOUYIN_CLIENT_CERT":    "cc",
				"DOUYIN_PRIVATE_KEY":    "pk",
				"DOUYIN_DTRAIT_BLOB":    "db",
				"DOUYIN_SESSION_DTRAIT": "sd",
				"DOUYIN_PROXY":          "http://p",
				"DOUYIN_PORT":           ":19090",
				"AUTH_TOKEN":            "tok",
				"DOUYIN_HEADLESS":       "0",
			},
			want: Config{
				CookieString:  "ck",
				CookiesFile:   "/tmp/c.txt",
				Ticket:        "tk",
				TsSign:        "ts",
				ClientCert:    "cc",
				PrivateKey:    "pk",
				DtraitBlob:    "db",
				SessionDtrait: "sd",
				Proxy:         "http://p",
				Port:          ":19090",
				AuthToken:     "tok",
				Headless:      false,
			},
		},
		{
			name: "仅别名回退",
			env: map[string]string{
				"DY_COOKIES":        "ck2",
				"DY_TICKET":         "tk2",
				"DY_TS_SIGN":        "ts2",
				"DY_CLIENT_CERT":    "cc2",
				"DY_PRIVATE_KEY":    "pk2",
				"DY_DTRAIT_BLOB":    "db2",
				"DY_SESSION_DTRAIT": "sd2",
				"DY_PROXY":          "http://p2",
				"DOUYIN_AUTH_TOKEN": "tok2",
			},
			want: Config{
				CookieString:  "ck2",
				Ticket:        "tk2",
				TsSign:        "ts2",
				ClientCert:    "cc2",
				PrivateKey:    "pk2",
				DtraitBlob:    "db2",
				SessionDtrait: "sd2",
				Proxy:         "http://p2",
				Port:          ":18080",
				AuthToken:     "tok2",
				Headless:      true,
			},
		},
		{
			name: "主键优先于别名",
			env: map[string]string{
				"DOUYIN_COOKIES":        "primary",
				"DY_COOKIES":            "alias",
				"DOUYIN_TICKET":         "primary",
				"DY_TICKET":             "alias",
				"DOUYIN_TS_SIGN":        "primary",
				"DY_TS_SIGN":            "alias",
				"DOUYIN_CLIENT_CERT":    "primary",
				"DY_CLIENT_CERT":        "alias",
				"DOUYIN_PRIVATE_KEY":    "primary",
				"DY_PRIVATE_KEY":        "alias",
				"DOUYIN_DTRAIT_BLOB":    "primary",
				"DY_DTRAIT_BLOB":        "alias",
				"DOUYIN_SESSION_DTRAIT": "primary",
				"DY_SESSION_DTRAIT":     "alias",
				"DOUYIN_PROXY":          "primary",
				"DY_PROXY":              "alias",
				"HTTPS_PROXY":           "fallback",
				"AUTH_TOKEN":            "primary",
				"DOUYIN_AUTH_TOKEN":     "alias",
			},
			want: Config{
				CookieString:  "primary",
				Ticket:        "primary",
				TsSign:        "primary",
				ClientCert:    "primary",
				PrivateKey:    "primary",
				DtraitBlob:    "primary",
				SessionDtrait: "primary",
				Proxy:         "primary",
				Port:          ":18080",
				AuthToken:     "primary",
				Headless:      true,
			},
		},
		{
			name: "代理回退到 HTTPS_PROXY",
			env:  map[string]string{"HTTPS_PROXY": "http://env-proxy"},
			want: Config{Proxy: "http://env-proxy", Port: ":18080", Headless: true},
		},
		{
			name: "DY_PROXY 优先于 HTTPS_PROXY",
			env: map[string]string{
				"DY_PROXY":    "http://dy",
				"HTTPS_PROXY": "http://https",
			},
			want: Config{Proxy: "http://dy", Port: ":18080", Headless: true},
		},
		{
			name: "收紧空白",
			env: map[string]string{
				"DOUYIN_COOKIES": "  spaced  ",
				"DOUYIN_PORT":    "  :1234  ",
			},
			want: Config{CookieString: "spaced", Port: ":1234", Headless: true},
		},
		{
			name: "Headless=1 为真",
			env:  map[string]string{"DOUYIN_HEADLESS": "1"},
			want: defaultConfig(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			applyEnv(t, tc.env)
			got := Load()
			if *got != tc.want {
				t.Fatalf("Load() = %+v, want %+v", *got, tc.want)
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	const key = "DOUYIN_TEST_ENVBOOL"
	tests := []struct {
		name string
		val  string
		def  bool
		want bool
	}{
		{"未设置取默认 true", "", true, true},
		{"未设置取默认 false", "", false, false},
		{"1 为真", "1", false, true},
		{"true 为真", "true", false, true},
		{"TRUE 忽略大小写", "TRUE", false, true},
		{"0 为假", "0", true, false},
		{"false 为假", "false", true, false},
		{"空白包裹的 0", "  0  ", true, false},
		{"非法值取默认 true", "notabool", true, true},
		{"非法值取默认 false", "notabool", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(key, tc.val)
			if got := EnvBool(key, tc.def); got != tc.want {
				t.Fatalf("EnvBool(%q, %v) = %v, want %v", tc.val, tc.def, got, tc.want)
			}
		})
	}
}
