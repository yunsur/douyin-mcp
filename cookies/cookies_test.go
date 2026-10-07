package cookies

import (
	"os"
	"path/filepath"
	"testing"
)

// clearCookieEnv 清空影响路径解析的两个环境变量。
func clearCookieEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DOUYIN_COOKIES_FILE", "")
	t.Setenv("DY_COOKIES_FILE", "")
}

func TestGetCookiesFilePath(t *testing.T) {
	t.Run("主键优先于别名", func(t *testing.T) {
		clearCookieEnv(t)
		t.Setenv("DOUYIN_COOKIES_FILE", "/tmp/douyin.txt")
		t.Setenv("DY_COOKIES_FILE", "/tmp/dy.txt")
		if got := GetCookiesFilePath(); got != "/tmp/douyin.txt" {
			t.Fatalf("GetCookiesFilePath() = %q, want %q", got, "/tmp/douyin.txt")
		}
	})

	t.Run("仅别名时回退别名", func(t *testing.T) {
		clearCookieEnv(t)
		t.Setenv("DY_COOKIES_FILE", "/tmp/dy.txt")
		if got := GetCookiesFilePath(); got != "/tmp/dy.txt" {
			t.Fatalf("GetCookiesFilePath() = %q, want %q", got, "/tmp/dy.txt")
		}
	})

	t.Run("未设置时回退当前目录 cookies.txt", func(t *testing.T) {
		clearCookieEnv(t)
		dir := t.TempDir()
		t.Chdir(dir)
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		want := filepath.Join(wd, "cookies.txt")
		if got := GetCookiesFilePath(); got != want {
			t.Fatalf("GetCookiesFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("收紧空白", func(t *testing.T) {
		clearCookieEnv(t)
		t.Setenv("DOUYIN_COOKIES_FILE", "  /tmp/x.txt  ")
		if got := GetCookiesFilePath(); got != "/tmp/x.txt" {
			t.Fatalf("GetCookiesFilePath() = %q, want %q", got, "/tmp/x.txt")
		}
	})
}

func TestLoadCookieTrimsAndStripsNewlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")
	raw := "  sessionid=abc;\n  tt_webid=42  \n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got, want := LoadCookie(path), "sessionid=abc;  tt_webid=42"; got != want {
		t.Fatalf("LoadCookie() = %q, want %q", got, want)
	}
}

func TestLoadCookieMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.txt")
	if got := LoadCookie(path); got != "" {
		t.Fatalf("LoadCookie() = %q, want empty", got)
	}
}

func TestSaveAndLoadCookieRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cookies.txt")
	// 目标目录不存在，SaveCookie 应报错且不创建文件。
	if err := SaveCookie(path, "value"); err == nil {
		t.Fatalf("SaveCookie() 到不存在目录应报错")
	}
	if Exists(path) {
		t.Fatalf("SaveCookie() 失败后不应存在文件")
	}

	path = filepath.Join(t.TempDir(), "cookies.txt")
	if Exists(path) {
		t.Fatalf("保存前 Exists() = true, want false")
	}
	if err := SaveCookie(path, "  sessionid=abc;  "); err != nil {
		t.Fatalf("SaveCookie: %v", err)
	}
	if !Exists(path) {
		t.Fatalf("保存后 Exists() = false, want true")
	}
	if got, want := LoadCookie(path), "sessionid=abc;"; got != want {
		t.Fatalf("LoadCookie() = %q, want %q", got, want)
	}

	// SaveCookie 落盘时收紧空白。
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := string(data), "sessionid=abc;"; got != want {
		t.Fatalf("文件内容 = %q, want %q", got, want)
	}

	// 文件权限不应向组/其他用户开放。
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("文件权限 = %o, 组/其他用户不应可读写", perm)
	}
}

func TestSaveCookieEmptyPathUsesEnv(t *testing.T) {
	clearCookieEnv(t)
	path := filepath.Join(t.TempDir(), "env-cookies.txt")
	t.Setenv("DOUYIN_COOKIES_FILE", path)

	if err := SaveCookie("", "  envvalue  "); err != nil {
		t.Fatalf("SaveCookie: %v", err)
	}
	if got, want := LoadCookie(""), "envvalue"; got != want {
		t.Fatalf("LoadCookie(\"\") = %q, want %q", got, want)
	}
}

func TestDeleteCookieIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")

	// 文件不存在时删除应成功返回。
	if err := DeleteCookie(path); err != nil {
		t.Fatalf("删除不存在文件: %v", err)
	}

	if err := SaveCookie(path, "value"); err != nil {
		t.Fatalf("SaveCookie: %v", err)
	}
	if !Exists(path) {
		t.Fatalf("保存后 Exists() = false, want true")
	}
	if err := DeleteCookie(path); err != nil {
		t.Fatalf("删除文件: %v", err)
	}
	if Exists(path) {
		t.Fatalf("删除后 Exists() = true, want false")
	}
	// 再次删除仍应成功。
	if err := DeleteCookie(path); err != nil {
		t.Fatalf("重复删除: %v", err)
	}
}
