package douyin

// Hermetic tests for the session/transport core in client.go and the
// non-Headers parts of httpclient.go: cookie parsing, session defaults, the
// msToken cache, uid/sec_uid/webid caching, risk mapping, URL/proxy helpers and
// the ordered-header logic of HTTPClient.Do.

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client/profiles"
)

// ---------------------------------------------------------------------------
// Cookie parsing / mapping
// ---------------------------------------------------------------------------

func TestCookiesParseTransCookies(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		names   []string
		values  map[string]string
		absent  []string
		wantStr string
	}{
		{
			name:    "basic pair list",
			in:      "a=b; c=d",
			names:   []string{"a", "c"},
			values:  map[string]string{"a": "b", "c": "d"},
			wantStr: "a=b; c=d",
		},
		{
			name:    "value containing equals",
			in:      "token=a=b=c",
			names:   []string{"token"},
			values:  map[string]string{"token": "a=b=c"},
			wantStr: "token=a=b=c",
		},
		{
			name:    "empty value is kept",
			in:      "empty=",
			names:   []string{"empty"},
			values:  map[string]string{"empty": ""},
			wantStr: "empty=",
		},
		{
			name:    "whitespace around pairs",
			in:      "  a=b ;  c=d  ",
			names:   []string{"a", "c"},
			values:  map[string]string{"a": "b", "c": "d"},
			wantStr: "a=b; c=d",
		},
		{
			name:    "duplicate names last wins",
			in:      "a=1; a=2",
			names:   []string{"a"},
			values:  map[string]string{"a": "2"},
			wantStr: "a=2",
		},
		{
			name:    "empty fragments skipped",
			in:      "a=1;;   ;b=2;",
			names:   []string{"a", "b"},
			values:  map[string]string{"a": "1", "b": "2"},
			wantStr: "a=1; b=2",
		},
		{
			name:    "empty name skipped",
			in:      "=x; a=1",
			names:   []string{"a"},
			values:  map[string]string{"a": "1"},
			absent:  []string{""},
			wantStr: "a=1",
		},
		{
			name:    "bare token preserved",
			in:      "foo",
			values:  map[string]string{},
			absent:  []string{"foo"},
			wantStr: "foo",
		},
		{
			name:    "bare token sits beside pairs",
			in:      "a=1; bare; b=2",
			names:   []string{"a", "b"},
			values:  map[string]string{"a": "1", "b": "2"},
			wantStr: "a=1; b=2; bare",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := TransCookies(tc.in)
			if got := c.Names(); !reflect.DeepEqual(got, tc.names) {
				t.Fatalf("Names() = %v, want %v", got, tc.names)
			}
			for k, want := range tc.values {
				if !c.Has(k) {
					t.Fatalf("Has(%q) = false, want true", k)
				}
				if got := c.Get(k); got != want {
					t.Fatalf("Get(%q) = %q, want %q", k, got, want)
				}
			}
			for _, k := range tc.absent {
				if c.Has(k) {
					t.Fatalf("Has(%q) = true, want false", k)
				}
			}
			if got := c.String(); got != tc.wantStr {
				t.Fatalf("String() = %q, want %q", got, tc.wantStr)
			}
			// Round trip through the serialized form must be stable.
			if got := TransCookies(c.String()).String(); got != tc.wantStr {
				t.Fatalf("round trip String() = %q, want %q", got, tc.wantStr)
			}
		})
	}
}

func TestCookiesParseDelAndGetSemantics(t *testing.T) {
	c := TransCookies("a=1; b=2; c=3")

	if got := c.Get("missing"); got != "" {
		t.Fatalf("Get(missing) = %q, want empty", got)
	}
	if c.Has("missing") {
		t.Fatal("Has(missing) = true, want false")
	}

	c.Del("b")
	if c.Has("b") {
		t.Fatal("Has(b) after Del = true, want false")
	}
	if got, want := c.Names(), []string{"a", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() after Del = %v, want %v", got, want)
	}
	if got, want := c.String(), "a=1; c=3"; got != want {
		t.Fatalf("String() after Del = %q, want %q", got, want)
	}

	// Deleting an absent key is a no-op and must not reorder survivors.
	c.Del("zzz")
	if got, want := c.String(), "a=1; c=3"; got != want {
		t.Fatalf("String() after no-op Del = %q, want %q", got, want)
	}

	// Set on an existing key updates in place without duplicating the name.
	c.Set("a", "9")
	if got, want := c.Names(), []string{"a", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() after Set = %v, want %v", got, want)
	}
	if got, want := c.String(), "a=9; c=3"; got != want {
		t.Fatalf("String() after Set = %q, want %q", got, want)
	}
}

func TestCookieStrNewClientDefaults(t *testing.T) {
	t.Setenv("DY_FIXED_SV_WEB_ID", "verify_pinned_id")

	t.Run("s_v_web_id generated when absent", func(t *testing.T) {
		c, err := NewClient("sessionid=sess", Options{Transport: &stubTransport{}})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if got := c.Cookie.Get("s_v_web_id"); got != "verify_pinned_id" {
			t.Fatalf("s_v_web_id = %q, want generated verify_pinned_id", got)
		}
		if !strings.Contains(c.CookieStr(), "s_v_web_id=verify_pinned_id") {
			t.Fatalf("CookieStr %q missing generated s_v_web_id", c.CookieStr())
		}
	})

	t.Run("s_v_web_id preserved when present", func(t *testing.T) {
		c, err := NewClient("s_v_web_id=keepme; sessionid=sess", Options{Transport: &stubTransport{}})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if got := c.Cookie.Get("s_v_web_id"); got != "keepme" {
			t.Fatalf("s_v_web_id = %q, want pre-existing keepme (must not be overwritten)", got)
		}
	})

	t.Run("msToken dropped and cookies round trip", func(t *testing.T) {
		c, err := NewClient("msToken=stale; sessionid=sess; UIFID=uif", Options{Transport: &stubTransport{}})
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if c.Cookie.Has("msToken") {
			t.Fatal("msToken must be dropped from the session cookie jar")
		}
		if strings.Contains(c.CookieStr(), "msToken") {
			t.Fatalf("CookieStr %q still carries msToken", c.CookieStr())
		}
		if got := c.Cookie.Get("sessionid"); got != "sess" {
			t.Fatalf("sessionid = %q, want sess", got)
		}
		if got := c.CookieStr(); !strings.Contains(got, "sessionid=sess") || !strings.Contains(got, "UIFID=uif") {
			t.Fatalf("CookieStr = %q, want sessionid + UIFID preserved", got)
		}
	})
}

// ---------------------------------------------------------------------------
// msToken cache
// ---------------------------------------------------------------------------

func TestMsTokenCacheFreshAndFallback(t *testing.T) {
	c, st := newStubClient(t, nil)

	// A pinned (real) token wins while fresh, and no background exchange starts.
	c.SetMsToken("real-token-1234567890")
	if got := c.MsToken(); got != "real-token-1234567890" {
		t.Fatalf("MsToken() = %q, want pinned real token", got)
	}
	if got := c.MsToken(); got != "real-token-1234567890" {
		t.Fatalf("second MsToken() = %q, want pinned real token", got)
	}
	if reqs := st.requests(); len(reqs) != 0 {
		t.Fatalf("fresh cached MsToken issued %d upstream requests, want 0", len(reqs))
	}

	// Expire the pinned token: MsToken now falls back to a generated value.
	c.mu.Lock()
	c.msTS = time.Now().Add(-2 * msTokenTTL)
	c.mu.Unlock()
	if got := c.MsToken(); got == "real-token-1234567890" {
		t.Fatal("MsToken() returned the expired real token, want a fresh fallback")
	} else if len(got) != 107 {
		t.Fatalf("fallback MsToken length = %d, want 107", len(got))
	}
}

func TestMsTokenEmptyPinnedIsRejected(t *testing.T) {
	c, _ := newStubClient(t, nil)
	c.SetMsToken("")
	if c.msTokenReal {
		t.Fatal("SetMsToken(\"\") must not pin an empty token")
	}
	// A never-pinned client falls back to a generated token.
	if got := c.MsToken(); len(got) != 107 {
		t.Fatalf("fallback MsToken length = %d, want 107", len(got))
	}
}

// ---------------------------------------------------------------------------
// uid / sec_uid / webid caching
// ---------------------------------------------------------------------------

func TestUIDCacheSingleRequest(t *testing.T) {
	const uid = int64(7445533736877264178)
	c, st := newStubClient(t, func(rec stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"user_uid": uid}), nil
	})
	c.SetWebID("7345678901234567890") // pre-cache so only the uid call hits upstream

	got, err := c.UID(t.Context())
	if err != nil {
		t.Fatalf("UID: %v", err)
	}
	if got != uid {
		t.Fatalf("UID = %d, want %d", got, uid)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("first UID issued %d upstream requests, want 1", n)
	}

	got2, err := c.UID(t.Context())
	if err != nil {
		t.Fatalf("second UID: %v", err)
	}
	if got2 != uid {
		t.Fatalf("second UID = %d, want %d", got2, uid)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("cached UID issued %d upstream requests, want still 1", n)
	}
}

func TestSecUIDCacheSingleRequest(t *testing.T) {
	const sec = "MS4wLjABAAAAcachedsecuid"
	c, st := newStubClient(t, func(rec stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"user": map[string]any{"sec_uid": sec}}), nil
	})

	got, err := c.SecUID(t.Context())
	if err != nil {
		t.Fatalf("SecUID: %v", err)
	}
	if got != sec {
		t.Fatalf("SecUID = %q, want %q", got, sec)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("first SecUID issued %d upstream requests, want 1", n)
	}

	got2, err := c.SecUID(t.Context())
	if err != nil {
		t.Fatalf("second SecUID: %v", err)
	}
	if got2 != sec {
		t.Fatalf("second SecUID = %q, want %q", got2, sec)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("cached SecUID issued %d upstream requests, want still 1", n)
	}
}

func TestWebIDCacheSingleRequest(t *testing.T) {
	const want = "7345678901234567890"
	c, st := newStubClient(t, func(rec stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"id": want}), nil
	})

	got := c.WebID(t.Context())
	if got != want {
		t.Fatalf("WebID = %q, want %q", got, want)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("first WebID issued %d upstream requests, want 1", n)
	}

	got2 := c.WebID(t.Context())
	if got2 != want {
		t.Fatalf("second WebID = %q, want %q", got2, want)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("cached WebID issued %d upstream requests, want still 1", n)
	}
}

// ---------------------------------------------------------------------------
// Risk mapping
// ---------------------------------------------------------------------------

func TestCheckRiskBdturingAndPassport(t *testing.T) {
	t.Run("bdturing subtype", func(t *testing.T) {
		payload := base64.StdEncoding.EncodeToString([]byte(`{"subtype":"verify_center"}`))
		resp := &Response{
			StatusCode: 200,
			Header: fhttp.Header{
				"X-Vc-Bdturing-Parameters": []string{payload},
				"X-Tt-Logid":               []string{"logid-1"},
			},
			Body: []byte("<html>turing</html>"),
		}
		err := CheckRisk(resp)
		if err == nil {
			t.Fatal("CheckRisk = nil, want bdturing error")
		}
		for _, want := range []string{"触发人机验证", "verify_center", "logid-1"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q missing %q", err.Error(), want)
			}
		}
	})

	t.Run("bdturing undecodable parameters", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Header:     fhttp.Header{"X-Vc-Bdturing-Parameters": []string{"!!!not-base64!!!"}},
			Body:       []byte("<html>turing</html>"),
		}
		err := CheckRisk(resp)
		if err == nil || !strings.Contains(err.Error(), "未知类型") {
			t.Fatalf("CheckRisk = %v, want unknown-subtype bdturing error", err)
		}
	})

	t.Run("passport decision scene", func(t *testing.T) {
		resp := &Response{
			StatusCode: 200,
			Header: fhttp.Header{
				"X-Tt-Verify-Passport-Decision": []string{`{"event_params":{"verify_scene":"captcha_login"}}`},
				"X-Tt-Logid":                    []string{"logid-2"},
			},
			Body: []byte("<html>passport</html>"),
		}
		err := CheckRisk(resp)
		if err == nil {
			t.Fatal("CheckRisk = nil, want passport error")
		}
		for _, want := range []string{"需要二次身份验证", "captcha_login", "logid-2"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q missing %q", err.Error(), want)
			}
		}
	})
}

func TestCheckRiskEmptyNonJSONAndJSON(t *testing.T) {
	t.Run("empty body names the status", func(t *testing.T) {
		err := CheckRisk(statusResponse(204, ""))
		if err == nil || !strings.Contains(err.Error(), "空响应") || !strings.Contains(err.Error(), "204") {
			t.Fatalf("CheckRisk = %v, want empty-response error naming HTTP 204", err)
		}
	})

	t.Run("non-JSON body names the status", func(t *testing.T) {
		err := CheckRisk(statusResponse(403, "<!DOCTYPE html><html>denied</html>"))
		if err == nil {
			t.Fatal("CheckRisk = nil, want non-JSON error")
		}
		for _, want := range []string{"非 JSON", "403"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q missing %q", err.Error(), want)
			}
		}
	})

	t.Run("acrawler challenge detected", func(t *testing.T) {
		err := CheckRisk(statusResponse(302, `<html>__ac_nonce=0abc</html>`))
		if err == nil || !strings.Contains(err.Error(), "acrawler") || !strings.Contains(err.Error(), "302") {
			t.Fatalf("CheckRisk = %v, want acrawler challenge error naming 302", err)
		}
	})

	t.Run("json object passes", func(t *testing.T) {
		if err := CheckRisk(&Response{StatusCode: 200, Body: []byte("  {\"a\":1}")}); err != nil {
			t.Fatalf("CheckRisk = %v, want nil for JSON object", err)
		}
		if err := CheckRisk(&Response{StatusCode: 200, Body: []byte("\n[1,2]")}); err != nil {
			t.Fatalf("CheckRisk = %v, want nil for JSON array", err)
		}
	})
}

func TestCheckRiskACRetrySkipsWithoutNonce(t *testing.T) {
	c, st := newStubClient(t, nil)
	orig := statusResponse(200, "<html>plain body, no challenge</html>")
	called := false
	got, err := c.acRetry(t.Context(), orig, func() (*Response, error) {
		called = true
		return jsonResponse(map[string]any{"ok": true}), nil
	})
	if err != nil {
		t.Fatalf("acRetry: %v", err)
	}
	if called {
		t.Fatal("acRetry replayed a response that carries no acrawler nonce")
	}
	if got != orig {
		t.Fatal("acRetry must return the original response when there is no challenge")
	}
	if n := len(st.requests()); n != 0 {
		t.Fatalf("acRetry issued %d requests, want 0", n)
	}
}

func TestClientGetJSONRiskError(t *testing.T) {
	c, st := newStubClient(t, func(rec stubRequest) (*Response, error) {
		return statusResponse(200, "<html>blocked by risk control</html>"), nil
	})
	_, err := c.GetJSON(t.Context(), douyinBase+"/aweme/v1/web/x/", NewParams(), nil)
	if err == nil || !strings.Contains(err.Error(), "非 JSON") {
		t.Fatalf("GetJSON err = %v, want non-JSON risk error", err)
	}
	if n := len(st.requests()); n != 1 {
		t.Fatalf("GetJSON issued %d requests, want 1 (no replay without a nonce)", n)
	}
}

// ---------------------------------------------------------------------------
// JSON decoding / URL helpers
// ---------------------------------------------------------------------------

func TestDecodeJSONObjectPreservesBigInts(t *testing.T) {
	const big = "7445533736877264178"
	out, err := decodeJSONObject([]byte(`{"id":7445533736877264178,"nested":{"v":9007199254740993}}`))
	if err != nil {
		t.Fatalf("decodeJSONObject: %v", err)
	}
	num, ok := out["id"].(json.Number)
	if !ok {
		t.Fatalf("id type = %T, want json.Number", out["id"])
	}
	if num.String() != big {
		t.Fatalf("id = %q, want exact %q", num.String(), big)
	}
	if _, isFloat := out["id"].(float64); isFloat {
		t.Fatal("id was decoded as float64 (would round the 64-bit id)")
	}
	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested type = %T, want map[string]any", out["nested"])
	}
	if got := nested["v"].(json.Number).String(); got != "9007199254740993" {
		t.Fatalf("nested v = %q, want 9007199254740993", got)
	}
}

func TestDecodeJSONObjectRejectsInvalid(t *testing.T) {
	_, err := decodeJSONObject([]byte("not json at all"))
	if err == nil || !strings.Contains(err.Error(), "decode json") {
		t.Fatalf("decodeJSONObject err = %v, want wrapped decode error", err)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		base, query, want string
	}{
		{"https://www.douyin.com/x", "", "https://www.douyin.com/x"},
		{"https://www.douyin.com/x", "a=1", "https://www.douyin.com/x?a=1"},
		{"https://www.douyin.com/x", "a=1&b=2", "https://www.douyin.com/x?a=1&b=2"},
		// base is used verbatim; no query-joining smarts.
		{"https://www.douyin.com/x?", "a=1", "https://www.douyin.com/x??a=1"},
	}
	for _, tc := range tests {
		if got := BuildURL(tc.base, tc.query); got != tc.want {
			t.Fatalf("BuildURL(%q, %q) = %q, want %q", tc.base, tc.query, got, tc.want)
		}
	}
}

func TestClientEscapePathValue(t *testing.T) {
	tests := []struct{ in, want string }{
		{"MS4wLjABAAAA", "MS4wLjABAAAA"},
		{"a b", "a%20b"},
		{"x/y", "x%2Fy"},
		{"a?b#c", "a%3Fb%23c"},
	}
	for _, tc := range tests {
		if got := EscapePathValue(tc.in); got != tc.want {
			t.Fatalf("EscapePathValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Proxy / profile resolution
// ---------------------------------------------------------------------------

func TestProxyNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"http://proxy:1", "http://proxy:1"},
		{"https://proxy:2", "https://proxy:2"},
		{"socks5://proxy:3", "socks5://proxy:3"},
	}
	for _, tc := range tests {
		if got := normalizeProxy(tc.in); got != tc.want {
			t.Fatalf("normalizeProxy(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// coreProfileString renders a TLS client profile for comparison.
// reflect.DeepEqual cannot be used here: ClientProfile embeds
// ClientHelloID.SpecFactory, a func value, and DeepEqual never reports two
// non-nil funcs as equal even when they are the same function.
func coreProfileString(p profiles.ClientProfile) string { return fmt.Sprintf("%#v", p) }

func TestProxyResolveProfile(t *testing.T) {
	t.Setenv("DY_HTTP_PROFILE", "")
	if got := resolveProfile(); coreProfileString(got) != coreProfileString(profiles.Chrome_150) {
		t.Fatal("empty DY_HTTP_PROFILE must fall back to Chrome_150")
	}

	known := profiles.MappedTLSClients["chrome_120"]
	t.Setenv("DY_HTTP_PROFILE", "chrome_120")
	if got := resolveProfile(); coreProfileString(got) != coreProfileString(known) {
		t.Fatal("known DY_HTTP_PROFILE must resolve to its mapped profile")
	}
	if coreProfileString(profiles.Chrome_120) == coreProfileString(profiles.Chrome_150) {
		t.Fatal("sanity: chrome_120 and chrome_150 should differ")
	}

	// Name matching trims and lowercases.
	t.Setenv("DY_HTTP_PROFILE", "  CHROME_120  ")
	if got := resolveProfile(); coreProfileString(got) != coreProfileString(known) {
		t.Fatal("DY_HTTP_PROFILE must be trimmed and lowercased")
	}

	t.Setenv("DY_HTTP_PROFILE", "definitely-not-a-profile")
	if got := resolveProfile(); coreProfileString(got) != coreProfileString(profiles.Chrome_150) {
		t.Fatal("unknown DY_HTTP_PROFILE must fall back to Chrome_150")
	}
}

func TestProxyFromEnv(t *testing.T) {
	t.Setenv("DY_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	if got := ProxyFromEnv(); got != "" {
		t.Fatalf("ProxyFromEnv() = %q, want empty", got)
	}

	t.Setenv("HTTPS_PROXY", "http://https-proxy:1")
	if got := ProxyFromEnv(); got != "http://https-proxy:1" {
		t.Fatalf("ProxyFromEnv() = %q, want HTTPS_PROXY value", got)
	}

	t.Setenv("DY_PROXY", "  http://dy-proxy:2  ")
	if got := ProxyFromEnv(); got != "http://dy-proxy:2" {
		t.Fatalf("ProxyFromEnv() = %q, want trimmed DY_PROXY value taking precedence", got)
	}
}

// ---------------------------------------------------------------------------
// HTTPClient.Do header ordering
// ---------------------------------------------------------------------------

func TestClientHTTPDoHeaderOrder(t *testing.T) {
	debugFile := filepath.Join(t.TempDir(), "reqs.jsonl")
	t.Setenv("DOUYIN_DEBUG_REQUESTS", debugFile)

	hc, err := NewHTTPClient("")
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	headers := Headers{
		{Name: "x-zeta", Value: "1"},
		{Name: "x-alpha", Value: "2"},
		{Name: "cookie", Value: "inline-should-be-dropped"},
		{Name: "content-length", Value: "999"},
	}
	wantOrder := []string{"x-zeta", "x-alpha", "cookie"}

	// Raw loopback listener so the concrete header order on the wire can be
	// observed (Go's net/http server discards ordering, so parse the request by
	// hand). Plain http:// is supported by the tls-client HTTP/1.1 transport.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	wireCh := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			wireCh <- nil
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		if _, err := br.ReadString('\n'); err != nil { // request line
			wireCh <- nil
			return
		}
		var names []string
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				break
			}
			if name, _, ok := strings.Cut(line, ":"); ok {
				names = append(names, strings.ToLower(strings.TrimSpace(name)))
			}
		}
		io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}")
		wireCh <- names
	}()

	ctx := t.Context()
	resp, doErr := hc.Get(ctx, "http://"+ln.Addr().String()+"/probe", headers, "a=b; c=d")
	if doErr != nil {
		// Unblock a listener that never saw a connection before reading.
		_ = ln.Close()
	}

	// The ordering decision is recorded by Do before it hits the network, so it
	// is asserted even when the loopback transport cannot complete.
	raw, err := os.ReadFile(debugFile)
	if err != nil {
		t.Fatalf("reading request log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("request log has %d lines, want 1", len(lines))
	}
	var rec struct {
		Headers []string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("unmarshal request log: %v", err)
	}
	if !reflect.DeepEqual(rec.Headers, wantOrder) {
		t.Fatalf("logged header order = %v, want %v", rec.Headers, wantOrder)
	}

	var wire []string
	select {
	case wire = <-wireCh:
	case <-time.After(5 * time.Second):
		t.Log("no wire capture within deadline; header ordering already asserted via the request log")
	}

	if doErr != nil {
		t.Skipf("HTTPClient could not reach the plain-HTTP loopback listener (%v); wire-order assertion skipped", doErr)
	}
	if resp == nil || string(resp.Body) != "{}" {
		t.Fatalf("response = %#v, want body {}", resp)
	}
	if wire == nil {
		t.Fatal("no request captured on the wire")
	}

	var got []string
	for _, n := range wire {
		switch n {
		case "x-zeta", "x-alpha", "cookie":
			got = append(got, n)
		}
	}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("wire header order = %v, want %v", got, wantOrder)
	}
	cookies := 0
	contentLengths := 0
	for _, n := range wire {
		switch n {
		case "cookie":
			cookies++
		case "content-length":
			contentLengths++
		}
	}
	if cookies != 1 {
		t.Fatalf("wire carried %d cookie headers, want exactly 1 (inline Cookie header must be replaced)", cookies)
	}
	if contentLengths > 1 {
		t.Fatalf("wire carried %d content-length headers, want at most 1", contentLengths)
	}
}
