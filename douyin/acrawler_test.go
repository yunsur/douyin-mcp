package douyin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractACNonce(t *testing.T) {
	cases := map[string]string{
		`<script>__ac_nonce="abc123"</script>`: "abc123",
		`__ac_nonce='abc123';`:                 "abc123",
		`{"__ac_nonce":"abc123"}`:              "abc123",
		`__ac_nonce = abc123; path=/`:          "abc123",
		`no nonce here`:                        "",
	}
	for body, want := range cases {
		if got := extractACNonce(body); got != want {
			t.Errorf("extractACNonce(%q) = %q want %q", body, got, want)
		}
	}
}

// writeNodeShim puts a fake `node` on PATH that echoes a JSON payload built
// from the AC_* environment variables the runner receives. This verifies our
// process plumbing (env, temp files, stdout parsing) without needing Node.
func writeNodeShim(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"{\\\"sig\\\":\\\"_${AC_NONCE}\\\",\\\"cookie\\\":\\\"$AC_COOKIE_ONLY; __ac_signature=_${AC_NONCE}\\\",\\\"variant\\\":\\\"${AC_VARIANT}\\\"}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "node"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGenerateACSignaturePlumbing(t *testing.T) {
	writeNodeShim(t)
	res, err := GenerateACSignature(t.Context(), ACOptions{
		Nonce:  "nonce42",
		Cookie: "a=b",
		Strict: true,
	})
	if err != nil {
		t.Fatalf("GenerateACSignature: %v", err)
	}
	if res["sig"] != "_nonce42" {
		t.Fatalf("sig = %v", res["sig"])
	}
	if res["provenance"] != "node_page_js" {
		t.Fatalf("provenance = %v", res["provenance"])
	}
	if res["variant"] != DefaultACVariant {
		t.Fatalf("variant = %v", res["variant"])
	}
	if cookies, ok := res["cookie"].(map[string]any); !ok || cookies["__ac_signature"] != "_nonce42" {
		t.Fatalf("cookie = %v", res["cookie"])
	}
	if muts, ok := res["cookie_mutations"].(map[string]any); !ok || muts["__ac_signature"] != "_nonce42" {
		t.Fatalf("cookie_mutations = %v", res["cookie_mutations"])
	}
}

func TestSolveACChallengeMergesCookie(t *testing.T) {
	writeNodeShim(t)
	c, err := NewClient("a=b", Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.SolveACChallenge(t.Context(), []byte(`<script>__ac_nonce="nonce7"</script>`))
	if err != nil {
		t.Fatalf("SolveACChallenge: %v", err)
	}
	if res["sig"] != "_nonce7" {
		t.Fatalf("sig = %v", res["sig"])
	}
	if got := c.Cookie.Get("__ac_signature"); got != "_nonce7" {
		t.Fatalf("会话 cookie 未合并 __ac_signature: %q", got)
	}
}

func TestGenerateACSignatureSoftWithoutNode(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no node anywhere
	res, err := GenerateACSignature(t.Context(), ACOptions{Nonce: "x", Cookie: "a=b"})
	if err != nil {
		t.Fatalf("非 strict 模式不应报错: %v", err)
	}
	if res["sig"] != "" || res["provenance"] != "unproven_synthetic" {
		t.Fatalf("soft 结果不对: %v", res)
	}
	if _, err := GenerateACSignature(t.Context(), ACOptions{Nonce: "x", Strict: true}); err == nil ||
		!strings.Contains(err.Error(), "Node.js is required") {
		t.Fatalf("strict 模式应报缺少 Node: %v", err)
	}
}

func TestACRetryOnlyOnChallenge(t *testing.T) {
	// No node → solving fails; the original response must be returned as-is.
	t.Setenv("PATH", t.TempDir())
	c, err := NewClient("a=b", Options{})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	send := func() (*Response, error) {
		calls++
		return &Response{StatusCode: 200, Body: []byte(`{"status_code":0}`)}, nil
	}

	plain := &Response{StatusCode: 200, Body: []byte(`{"ok":true}`)}
	got, err := c.acRetry(t.Context(), plain, send)
	if err != nil || got != plain || calls != 0 {
		t.Fatalf("普通响应不应重试: got=%v err=%v calls=%d", got, err, calls)
	}

	challenge := &Response{StatusCode: 200, Body: []byte(`<script>__ac_nonce="n1"</script>`)}
	got, err = c.acRetry(t.Context(), challenge, send)
	if err != nil || got != challenge || calls != 0 {
		t.Fatalf("无法求解时应原样返回: got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestACRetryReplaysWhenSolvable(t *testing.T) {
	writeNodeShim(t)
	c, err := NewClient("a=b", Options{})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	retried := &Response{StatusCode: 200, Body: []byte(`{"ok":true}`)}
	send := func() (*Response, error) { calls++; return retried, nil }
	challenge := &Response{StatusCode: 200, Body: []byte(`<script>__ac_nonce="n2"</script>`)}
	got, err := c.acRetry(t.Context(), challenge, send)
	if err != nil {
		t.Fatal(err)
	}
	if got != retried || calls != 1 {
		t.Fatalf("求解成功后应重放一次: got=%v calls=%d", got, calls)
	}
	if c.Cookie.Get("__ac_signature") != "_n2" {
		t.Fatalf("__ac_signature 未合并: %q", c.Cookie.Get("__ac_signature"))
	}
}
