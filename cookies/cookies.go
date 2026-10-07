// Package cookies manages the on-disk Douyin cookie store.
package cookies

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultCookieFileName = "cookies.txt"

// GetCookiesFilePath returns the configured cookie file path.
func GetCookiesFilePath() string {
	if p := strings.TrimSpace(os.Getenv("DOUYIN_COOKIES_FILE")); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv("DY_COOKIES_FILE")); p != "" {
		return p
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, defaultCookieFileName)
	}
	return defaultCookieFileName
}

// LoadCookie returns the cookie header string from disk ("" when absent).
func LoadCookie(path string) string {
	if path == "" {
		path = GetCookiesFilePath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\n", ""))
}

// SaveCookie persists the cookie header string.
func SaveCookie(path, value string) error {
	if path == "" {
		path = GetCookiesFilePath()
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(value)), 0o600)
}

// DeleteCookie removes the cookie file, resetting the login state.
func DeleteCookie(path string) error {
	if path == "" {
		path = GetCookiesFilePath()
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Exists reports whether a cookie file is present.
func Exists(path string) bool {
	if path == "" {
		path = GetCookiesFilePath()
	}
	_, err := os.Stat(path)
	return err == nil
}
