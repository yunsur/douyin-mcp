package douyin

import (
	"strings"
	"testing"
)

// addPageCookies seeds the anonymous page cookies (fpk1/fpk2 included) before
// the passport flow; it must work fully offline.
func TestAddPageCookiesSeedsFingerprintCookies(t *testing.T) {
	c, err := NewClient("ttwid=fake", Options{})
	if err != nil {
		t.Fatal(err)
	}
	c.addPageCookies()

	if got, want := c.Cookie.Get("fpk2"), BuildFpk2(GetProfile().UA); got != want {
		t.Fatalf("fpk2 = %q want %q", got, want)
	}
	fpk1 := c.Cookie.Get("fpk1")
	if fpk1 == "" {
		t.Fatal("fpk1 未生成")
	}
	if !strings.HasPrefix(fpk1, "U2FsdGVkX1") { // base64("Salted__")
		t.Fatalf("fpk1 不是 OpenSSL 加密容器: %q", fpk1[:12])
	}
	if c.Cookie.Get("s_v_web_id") == "" {
		t.Fatal("s_v_web_id 未生成")
	}
}
