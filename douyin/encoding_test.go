package douyin

// Hermetic unit tests for the query/URL/header/JSON encoding helpers in
// params.go, params_ext.go, headers.go, strdata.go and util.go.
//
// Everything here is offline and deterministic: no network, no clock
// dependence, no real browsers.

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers (unique to this file)
// ---------------------------------------------------------------------------

// encodingJNodeKeys returns an object node's keys in insertion order.
func encodingJNodeKeys(n *jNode) []string {
	out := make([]string, 0, len(n.pairs))
	for _, p := range n.pairs {
		out = append(out, p.key)
	}
	return out
}

// encodingHeaderNames returns the header names in order.
func encodingHeaderNames(h Headers) []string {
	out := make([]string, 0, len(h))
	for _, kv := range h {
		out = append(out, kv.Name)
	}
	return out
}

// encodingDecodeCustomB64 decodes the mssdk custom-alphabet base64 used by
// EncodeStrData (standard '=' padding).
func encodingDecodeCustomB64(t *testing.T, s string) []byte {
	t.Helper()
	rev := make(map[byte]byte, len(customAlphabet))
	for i := range len(customAlphabet) {
		rev[customAlphabet[i]] = byte(i)
	}
	var out []byte
	var acc uint32
	n := 0
	for i := range len(s) {
		c := s[i]
		if c == '=' {
			break
		}
		v, ok := rev[c]
		if !ok {
			t.Fatalf("invalid custom base64 byte %q in %q", c, s)
		}
		acc = acc<<6 | uint32(v)
		if n++; n == 4 {
			out = append(out, byte(acc>>16), byte(acc>>8), byte(acc))
			acc, n = 0, 0
		}
	}
	switch n {
	case 2:
		out = append(out, byte(acc<<12>>16))
	case 3:
		acc <<= 6
		out = append(out, byte(acc>>16), byte(acc>>8))
	case 1:
		t.Fatalf("truncated custom base64 group in %q", s)
	}
	return out
}

// ---------------------------------------------------------------------------
// params.go
// ---------------------------------------------------------------------------

func TestParamsOrderReplaceAndToString(t *testing.T) {
	empty := NewParams()
	if empty.Len() != 0 || empty.ToString() != "" || empty.SpliceURL() != "" {
		t.Fatalf("empty Params not empty: len=%d tostring=%q splice=%q",
			empty.Len(), empty.ToString(), empty.SpliceURL())
	}

	p := NewParams()
	p.Add("a", "1").Add("b", "2").Add("a", "3")
	if got, want := p.Len(), 2; got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	if got, want := p.Keys(), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys = %v, want %v (replace must keep position)", got, want)
	}
	if v, ok := p.Get("a"); !ok || v != "3" {
		t.Fatalf(`Get("a") = (%q, %v), want ("3", true)`, v, ok)
	}
	if _, ok := p.Get("zz"); ok {
		t.Fatal(`Get("zz") reported present`)
	}
	if got, want := p.ToString(), "a=3&b=2"; got != want {
		t.Fatalf("ToString = %q, want %q", got, want)
	}

	p.Del("a")
	p.Del("missing") // no-op
	if got, want := p.Keys(), []string{"b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys after Del = %v, want %v", got, want)
	}
	if got, want := p.ToString(), "b=2"; got != want {
		t.Fatalf("ToString after Del = %q, want %q", got, want)
	}
	if _, ok := p.Get("a"); ok {
		t.Fatal("deleted key still present")
	}
}

func TestParamsMergeAndClone(t *testing.T) {
	base := NewParams().Add("a", "A").Add("b", "B")
	other := NewParams().Add("b", "B2").Add("c", "C")
	base.Merge(other)
	if got, want := base.Keys(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge keys = %v, want %v", got, want)
	}
	if v, _ := base.Get("b"); v != "B2" {
		t.Fatalf("Merge did not overwrite b: %q", v)
	}
	if v, _ := base.Get("c"); v != "C" {
		t.Fatalf("Merge did not add c: %q", v)
	}

	cp := base.Clone()
	cp.Add("b", "CHANGED").Add("d", "D")
	if v, _ := base.Get("b"); v != "B2" {
		t.Fatalf("Clone shared value: base.b = %q", v)
	}
	if _, ok := base.Get("d"); ok {
		t.Fatal("Clone shared key d with original")
	}
	base.Add("c", "BASE-ONLY")
	if v, _ := cp.Get("c"); v != "C" {
		t.Fatalf("Clone shared value after original mutation: cp.c = %q", v)
	}
}

func TestSpliceURLEncodesValuesOnly(t *testing.T) {
	p := NewParams().
		Add("plain", "abcXYZ019").
		Add("safe", "-_.~").
		Add("space", "a b").
		Add("plus", "a+b").
		Add("slash", "a/b").
		Add("pct", "50%").
		Add("cjk", "中文").
		Add("emoji", "😀").
		Add("empty", "").
		Add("already", "a%20b").
		Add("key with space", "v")

	want := "plain=abcXYZ019" +
		"&safe=-_.~" +
		"&space=a%20b" +
		"&plus=a%2Bb" +
		"&slash=a%2Fb" +
		"&pct=50%25" +
		"&cjk=%E4%B8%AD%E6%96%87" +
		"&emoji=%F0%9F%98%80" +
		"&empty=" +
		"&already=a%2520b" +
		"&key with space=v"
	if got := p.SpliceURL(); got != want {
		t.Fatalf("SpliceURL = %q\nwant          %q", got, want)
	}
}

func TestQuoteStrictAndQuotePlusDivergeOnSpace(t *testing.T) {
	cases := []struct {
		in           string
		strict, plus string
	}{
		{"", "", ""},
		{"abcXYZ019-_.~", "abcXYZ019-_.~", "abcXYZ019-_.~"},
		{"a b", "a%20b", "a+b"},
		{"a+b", "a%2Bb", "a%2Bb"},
		{"a/b", "a%2Fb", "a%2Fb"},
		{"50%", "50%25", "50%25"},
		{"中文", "%E4%B8%AD%E6%96%87", "%E4%B8%AD%E6%96%87"},
		{"😀", "%F0%9F%98%80", "%F0%9F%98%80"},
		{"a%20b", "a%2520b", "a%2520b"},
	}
	for _, tc := range cases {
		if got := quoteStrict(tc.in); got != tc.strict {
			t.Errorf("quoteStrict(%q) = %q, want %q", tc.in, got, tc.strict)
		}
		if got := quotePlus(tc.in); got != tc.plus {
			t.Errorf("quotePlus(%q) = %q, want %q", tc.in, got, tc.plus)
		}
	}
}

func TestEncodeStandardQueryEncodesKeysAndValues(t *testing.T) {
	p := NewParams().Add("a b", "c/d").Add("k", "中 文").Add("e", "")
	want := "a+b=c%2Fd&k=%E4%B8%AD+%E6%96%87&e="
	if got := standardEncodeQuery(p); got != want {
		t.Fatalf("standardEncodeQuery = %q, want %q", got, want)
	}
	if got := standardEncodeQuery(NewParams()); got != "" {
		t.Fatalf("empty standardEncodeQuery = %q, want empty", got)
	}
}

func TestEncodeEscapeQuerySafeSetAndSpace(t *testing.T) {
	cases := []struct {
		in, safe, want string
	}{
		{"", "", ""},
		{"a b", "", "a%20b"}, // space is %20, never '+'
		{"a?b&c", "?&", "a?b&c"},
		{"a?b&c", "", "a%3Fb%26c"},
		{"a/b", "", "a%2Fb"},
		{"+-._~", "", "%2B-._~"},
		{"中", "", "%E4%B8%AD"},
		{"%", "%", "%"},
	}
	for _, tc := range cases {
		if got := escapeQuery(tc.in, tc.safe); got != tc.want {
			t.Errorf("escapeQuery(%q, %q) = %q, want %q", tc.in, tc.safe, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// params_ext.go
// ---------------------------------------------------------------------------

func TestParamsPlatformGroupShape(t *testing.T) {
	prof := GetProfile()
	p := NewParams().WithPlatform("12", "190500", "19.5.0")

	wantKeys := []string{
		"device_platform", "aid", "channel", "update_version_code", "pc_client_type",
		"pc_libra_divert", "support_h265", "support_dash", "cpu_core_num", "version_code",
		"version_name", "cookie_enabled", "screen_width", "screen_height", "browser_language",
		"browser_platform", "browser_name", "browser_version", "browser_online", "engine_name",
		"engine_version", "os_name", "os_version", "device_memory", "platform",
		"downlink", "effective_type", "round_trip_time",
	}
	if got := p.Keys(); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("WithPlatform keys = %v\nwant %v", got, wantKeys)
	}
	if p.Len() != len(wantKeys) {
		t.Fatalf("WithPlatform Len = %d, want %d", p.Len(), len(wantKeys))
	}
	want := map[string]string{
		"device_platform": "webapp", "aid": "6383", "channel": "channel_pc_web",
		"update_version_code": "170400", "pc_client_type": "1", "pc_libra_divert": "Windows",
		"support_h265": "1", "support_dash": "1", "cpu_core_num": prof.CpuCoreNum,
		"version_code": "190500", "version_name": "19.5.0", "cookie_enabled": "true",
		"screen_width": prof.ScreenWidth, "screen_height": prof.ScreenHeight,
		"browser_language": "zh-CN", "browser_platform": "Win32", "browser_name": prof.BrowserName,
		"browser_version": prof.BrowserVersion, "browser_online": "true", "engine_name": "Blink",
		"engine_version": prof.EngineVersion, "os_name": "Windows", "os_version": "10",
		"device_memory": prof.DeviceMemory, "platform": "PC", "downlink": "10",
		"effective_type": "4g", "round_trip_time": "12",
	}
	for k, v := range want {
		if got, _ := p.Get(k); got != v {
			t.Errorf("WithPlatform[%s] = %q, want %q", k, got, v)
		}
	}

	// Adding an existing key replaces the value but keeps its position.
	p.Add("aid", "other")
	if got := p.Keys(); !reflect.DeepEqual(got, wantKeys) {
		t.Fatalf("key order changed after replace: %v", got)
	}
	if v, _ := p.Get("aid"); v != "other" {
		t.Fatalf("aid = %q, want other", v)
	}
}

func TestParamsLiveAndCreatorGroupShape(t *testing.T) {
	prof := GetProfile()

	live := NewParams().WithLivePlatform("7")
	wantLive := []string{
		"update_version_code", "pc_client_type", "pc_libra_divert", "support_h265",
		"support_dash", "cpu_core_num", "version_code", "version_name", "cookie_enabled",
		"screen_width", "screen_height", "browser_language", "browser_platform",
		"browser_name", "browser_version", "browser_online", "engine_name", "engine_version",
		"os_name", "os_version", "device_memory", "platform", "downlink", "effective_type",
		"round_trip_time",
	}
	if got := live.Keys(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("WithLivePlatform keys = %v\nwant %v", got, wantLive)
	}
	for k, v := range map[string]string{
		"update_version_code": "170400", "pc_client_type": "1", "pc_libra_divert": "Windows",
		"support_h265": "1", "support_dash": "0", "cpu_core_num": prof.CpuCoreNum,
		"version_code": "320100", "version_name": "32.1.0", "round_trip_time": "7",
		"screen_width": prof.ScreenWidth, "browser_name": prof.BrowserName,
	} {
		if got, _ := live.Get(k); got != v {
			t.Errorf("WithLivePlatform[%s] = %q, want %q", k, got, v)
		}
	}

	creator := NewParams().WithCreatorPlatform()
	wantCreator := []string{
		"cookie_enabled", "screen_width", "screen_height", "browser_language",
		"browser_platform", "browser_name", "browser_version", "browser_online",
		"timezone_name", "aid", "support_h265",
	}
	if got := creator.Keys(); !reflect.DeepEqual(got, wantCreator) {
		t.Fatalf("WithCreatorPlatform keys = %v\nwant %v", got, wantCreator)
	}
	if v, _ := creator.Get("browser_name"); v != "Mozilla" {
		t.Errorf("creator browser_name = %q, want Mozilla", v)
	}
	wantVer := strings.Replace(prof.UA, "Mozilla/", "", 1)
	if v, _ := creator.Get("browser_version"); v != wantVer {
		t.Errorf("creator browser_version = %q, want %q", v, wantVer)
	}
	if v, _ := creator.Get("timezone_name"); v != "Asia/Shanghai" {
		t.Errorf("creator timezone_name = %q", v)
	}
	if v, _ := creator.Get("aid"); v != "1128" {
		t.Errorf("creator aid = %q, want 1128", v)
	}
	if _, ok := creator.Get("device_platform"); ok {
		t.Error("creator group must not carry device_platform")
	}
}

func TestParamsCookieDerivedGroups(t *testing.T) {
	c, _ := newStubClient(t, nil) // sessionid=abc; UIFID=uif (+ generated s_v_web_id)
	fp := c.Cookie.Get("s_v_web_id")
	if fp == "" {
		t.Fatal("stub client did not seed s_v_web_id")
	}

	p := NewParams()
	p.WithUIFID(c)
	p.WithVerifyFP(c)
	p.WithMsToken()
	c.SetWebID("7600000000000000001")
	p.WithWebID(t.Context(), c, "https://www.douyin.com/")

	if got, want := p.Keys(), []string{"uifid", "verifyFp", "fp", "msToken", "webid"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cookie-derived keys = %v, want %v", got, want)
	}
	if v, _ := p.Get("uifid"); v != "uif" {
		t.Errorf("uifid = %q, want uif", v)
	}
	if v, _ := p.Get("verifyFp"); v != fp {
		t.Errorf("verifyFp = %q, want %q", v, fp)
	}
	if v, _ := p.Get("fp"); v != fp {
		t.Errorf("fp = %q, want %q", v, fp)
	}
	if v, _ := p.Get("msToken"); len(v) != 107 {
		t.Errorf("msToken length = %d, want 107", len(v))
	}
	if v, _ := p.Get("webid"); v != "7600000000000000001" {
		t.Errorf("webid = %q, want pinned value", v)
	}

	// Absent cookies must add nothing at all.
	c.Cookie.Del("UIFID")
	c.Cookie.Del("s_v_web_id")
	empty := NewParams()
	empty.WithUIFID(c).WithVerifyFP(c)
	if empty.Len() != 0 {
		t.Fatalf("absent cookies added keys: %v", empty.Keys())
	}
}

func TestParamsAbogusAppendedLastAndReplacedInPlace(t *testing.T) {
	c, _ := newStubClient(t, nil)
	p := NewParams().Add("device_platform", "webapp")

	p.WithABogusHost(c, NewParams().Add("x", "1"), "live.douyin.com")
	if got, want := p.Keys(), []string{"device_platform", "a_bogus"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WithABogusHost keys = %v, want %v", got, want)
	}
	if sig, _ := p.Get("a_bogus"); sig == "" {
		t.Fatal("a_bogus signature is empty")
	}

	// Re-signing replaces the signature in place (no duplicate key).
	first, _ := p.Get("a_bogus")
	p.WithABogusHost(c, nil, "live.douyin.com")
	if got, want := p.Keys(), []string{"device_platform", "a_bogus"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("re-signing keys = %v, want %v", got, want)
	}
	if second, _ := p.Get("a_bogus"); second == "" || second == first {
		t.Fatalf("re-signing did not refresh signature (first=%q second=%q)", first, second)
	}
}

// ---------------------------------------------------------------------------
// headers.go (Headers itself lives in httpclient.go)
// ---------------------------------------------------------------------------

func TestHeadersSetGetDelCaseInsensitive(t *testing.T) {
	var h Headers
	if _, ok := h.Get("x"); ok {
		t.Fatal("Get on empty Headers reported present")
	}

	h.Set("Accept", "a")
	h.Set("accept", "b") // case-insensitive replace, preserves original spelling
	h.Set("content-type", "json")
	h.Set("X-Trace", "t")
	if len(h) != 3 {
		t.Fatalf("Set appended duplicates: len = %d, want 3", len(h))
	}
	if h[0].Name != "Accept" || h[0].Value != "b" {
		t.Fatalf("replace kept wrong slot: %+v", h[0])
	}
	if got, ok := h.Get("ACCEPT"); !ok || got != "b" {
		t.Fatalf(`Get("ACCEPT") = (%q, %v)`, got, ok)
	}
	if got, ok := h.Get("x-trace"); !ok || got != "t" {
		t.Fatalf(`Get("x-trace") = (%q, %v)`, got, ok)
	}

	h.Del("CONTENT-TYPE")
	if got, want := encodingHeaderNames(h), []string{"Accept", "X-Trace"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("after Del: %v, want %v", got, want)
	}
	h.Del("missing") // no-op
	if len(h) != 2 {
		t.Fatalf("Del of missing header changed len to %d", len(h))
	}
}

func TestHeadersAppendOrderAndReplaceInPlace(t *testing.T) {
	var h Headers
	for _, name := range []string{"a", "b", "c", "d"} {
		h.Set(name, "1")
	}
	h.Set("b", "2") // replace in place
	h.Set("e", "5") // append at end
	if got, want := encodingHeaderNames(h), []string{"a", "b", "c", "d", "e"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	for i, want := range []string{"1", "2", "1", "1", "5"} {
		if h[i].Value != want {
			t.Errorf("h[%d].Value = %q, want %q", i, h[i].Value, want)
		}
	}
}

func TestHeadersBuildHeadersKinds(t *testing.T) {
	prof := GetProfile()

	get := BuildHeaders(HeaderGET)
	wantGet := []string{
		"user-agent", "accept", "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform",
		"accept-language", "priority", "sec-fetch-dest", "sec-fetch-mode", "sec-fetch-site",
	}
	if got := encodingHeaderNames(get); !reflect.DeepEqual(got, wantGet) {
		t.Fatalf("GET header order = %v\nwant %v", got, wantGet)
	}
	for k, v := range map[string]string{
		"user-agent": prof.UA, "accept": "application/json, text/plain, */*",
		"sec-ch-ua": prof.SecCHUA, "sec-ch-ua-mobile": "?0",
		"sec-ch-ua-platform": prof.SecCHUAPlatform, "accept-language": prof.AcceptLanguage,
		"priority": "u=1, i", "sec-fetch-dest": "empty", "sec-fetch-mode": "cors",
		"sec-fetch-site": "same-origin",
	} {
		if got, _ := get.Get(k); got != v {
			t.Errorf("GET[%s] = %q, want %q", k, got, v)
		}
	}
	if _, ok := get.Get("content-type"); ok {
		t.Error("GET must not set content-type")
	}

	if v, _ := BuildHeaders(HeaderPOST).Get("accept"); v != "*/*" {
		t.Errorf("POST accept = %q, want */*", v)
	}
	if v, _ := BuildHeaders(HeaderPOST).Get("content-type"); v != "application/json; charset=UTF-8" {
		t.Errorf("POST content-type = %q", v)
	}
	if v, _ := BuildHeaders(HeaderFORM).Get("content-type"); v != "application/x-www-form-urlencoded; charset=UTF-8" {
		t.Errorf("FORM content-type = %q", v)
	}
	if v, _ := BuildHeaders(HeaderPROTOBUF).Get("accept"); v != "application/x-protobuf" {
		t.Errorf("PROTOBUF accept = %q", v)
	}
	if v, _ := BuildHeaders(HeaderPROTOBUF).Get("content-type"); v != "application/x-protobuf" {
		t.Errorf("PROTOBUF content-type = %q", v)
	}

	doc := BuildHeaders(HeaderDOC)
	wantDoc := []string{
		"accept", "accept-language", "cache-control", "pragma", "priority", "sec-ch-ua",
		"sec-ch-ua-mobile", "sec-ch-ua-platform", "sec-fetch-dest", "sec-fetch-mode",
		"sec-fetch-site", "sec-fetch-user", "upgrade-insecure-requests", "user-agent",
	}
	if got := encodingHeaderNames(doc); !reflect.DeepEqual(got, wantDoc) {
		t.Fatalf("DOC header order = %v\nwant %v", got, wantDoc)
	}
	for k, v := range map[string]string{
		"accept-language": prof.AcceptLanguage, "cache-control": "no-cache", "pragma": "no-cache",
		"priority": "u=0, i", "sec-ch-ua": prof.SecCHUA, "sec-fetch-dest": "document",
		"sec-fetch-mode": "navigate", "sec-fetch-site": "none", "sec-fetch-user": "?1",
		"upgrade-insecure-requests": "1", "user-agent": prof.UA,
	} {
		if got, _ := doc.Get(k); got != v {
			t.Errorf("DOC[%s] = %q, want %q", k, got, v)
		}
	}
}

func TestHeadersSetRefererUIFIDAndBDNoOp(t *testing.T) {
	var h Headers
	h.SetReferer("https://www.douyin.com/video/1")
	if v, _ := h.Get("referer"); v != "https://www.douyin.com/video/1" {
		t.Fatalf("referer = %q", v)
	}
	if h.WithUIFID(nil) == nil {
		t.Fatal("WithUIFID(nil) returned nil receiver")
	}

	c, _ := newStubClient(t, nil)
	h.WithUIFID(c)
	if v, _ := h.Get("uifid"); v != "uif" {
		t.Fatalf("uifid = %q, want uif", v)
	}
	c.Cookie.Del("UIFID")
	var noUID Headers
	noUID.WithUIFID(c)
	if _, ok := noUID.Get("uifid"); ok {
		t.Fatal("WithUIFID added uifid without cookie")
	}

	// Cookie-only mode (no ticket-guard key) is a no-op in both cases.
	var bd Headers
	bd.WithBDReadonly(nil)
	bd.WithBDReadonly(c)
	if len(bd) != 0 {
		t.Fatalf("WithBDReadonly without key added headers: %+v", bd)
	}
}

// ---------------------------------------------------------------------------
// strdata.go — JSON node model and compact serialization
// ---------------------------------------------------------------------------

func TestJNodeBuildersCompactAndOrder(t *testing.T) {
	root := jObj()
	root.set("s", jStr("hi"))
	root.set("n", jInt(-12))
	root.set("f", jNum("1.0"))
	root.set("b", jBool(true))
	root.set("z", jNull())
	arr := jArr()
	arr.arr = append(arr.arr, jInt(1), jStr("x"), jBool(false))
	root.set("a", arr)
	root.set("s", jStr("replaced")) // replaces in place, keeps first position

	want := `{"s":"replaced","n":-12,"f":1.0,"b":true,"z":null,"a":[1,"x",false]}`
	if got := root.stringCompact(); got != want {
		t.Fatalf("compact = %s\nwant %s", got, want)
	}
	for _, tc := range []struct {
		n    *jNode
		want string
	}{
		{jObj(), "{}"},
		{jArr(), "[]"},
		{jNull(), "null"},
		{jBool(false), "false"},
		{jStr(""), `""`},
	} {
		if got := tc.n.stringCompact(); got != tc.want {
			t.Errorf("compact(%c) = %q, want %q", tc.n.kind, got, tc.want)
		}
	}
}

func TestJNodeGetSetDefaultDelDeepCopy(t *testing.T) {
	root := jObj()
	if _, ok := root.get("x"); ok {
		t.Fatal("get on missing key reported present")
	}
	root.setDefault("x", jInt(1))
	root.setDefault("x", jInt(2)) // must be ignored
	if v, ok := root.get("x"); !ok || v.num != "1" {
		t.Fatalf("setDefault overwrote existing key: %+v", v)
	}
	root.set("x", jInt(3))
	if v, _ := root.get("x"); v.num != "3" {
		t.Fatalf("set did not replace: %+v", v)
	}
	root.setDefault("y", jStr("y")) // appended at end
	if got, want := encodingJNodeKeys(root), []string{"x", "y"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}

	if removed := root.del("x"); removed == nil || removed.num != "3" {
		t.Fatalf("del returned %+v, want 3", removed)
	}
	if root.del("missing") != nil {
		t.Fatal("del of missing key returned non-nil")
	}
	if got, want := encodingJNodeKeys(root), []string{"y"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys after del = %v, want %v", got, want)
	}

	if _, ok := jStr("s").get("k"); ok {
		t.Fatal("get on a string node reported present")
	}
	var nilNode *jNode
	if _, ok := nilNode.get("k"); ok {
		t.Fatal("get on nil node reported present")
	}
	if nilNode.deepCopy() != nil {
		t.Fatal("deepCopy of nil node is non-nil")
	}

	orig := jObj()
	a := jArr()
	a.arr = append(a.arr, jInt(1))
	orig.set("a", a)
	cp := orig.deepCopy()
	cpArr, ok := cp.get("a")
	if !ok || cpArr.kind != 'a' || len(cpArr.arr) != 1 {
		t.Fatalf("deepCopy lost array: %+v", cpArr)
	}
	cpArr.arr[0] = jInt(9)
	if v, _ := orig.get("a"); v.arr[0].num != "1" {
		t.Fatalf("deepCopy shared array element: %q", v.arr[0].num)
	}
	cp.set("a", jNull())
	if v, _ := orig.get("a"); v.kind != 'a' {
		t.Fatalf("deepCopy shared object slot: kind %c", v.kind)
	}
}

func TestJSONStringEscaping(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", `""`},
		{"plain", `"plain"`},
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"a\nb\rc\td", `"a\nb\rc\td"`},
		{"\b\f", `"\b\f"`},
		{"\x00", `"\u0000"`},
		{"\x01\x1f", `"\u0001\u001f"`},
		{"\x7f", "\"\x7f\""}, // DEL is not < 0x20: emitted raw
		{"中文😀", "\"中文😀\""},
	}
	for _, tc := range cases {
		var sb strings.Builder
		encodeJSONStr(&sb, tc.in)
		if got := sb.String(); got != tc.want {
			t.Errorf("encodeJSONStr(%q) = %s, want %s", tc.in, got, tc.want)
		}
		var back string
		if err := json.Unmarshal([]byte(sb.String()), &back); err != nil {
			t.Errorf("encodeJSONStr(%q) produced invalid JSON: %v", tc.in, err)
		} else if back != tc.in {
			t.Errorf("encodeJSONStr(%q) round-trip = %q", tc.in, back)
		}
	}
}

func TestJSONOrderedParsePreservesSpellingAndOrder(t *testing.T) {
	src := []byte(`{"z":1,"a":[true,null,"s","中"],"n":1.50,"big":12345678901234567890}`)
	n, err := parseOrderedJSON(src)
	if err != nil {
		t.Fatalf("parseOrderedJSON: %v", err)
	}
	if got := n.stringCompact(); got != string(src) {
		t.Fatalf("round-trip = %s\nwant      %s", got, src)
	}
	if got, want := encodingJNodeKeys(n), []string{"z", "a", "n", "big"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("key order = %v, want %v", got, want)
	}
	if _, err := parseOrderedJSON([]byte("{")); err == nil {
		t.Fatal("parseOrderedJSON accepted malformed input")
	}
}

func TestStrdataNumFromValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"int", 5, "5"},
		{"int64", int64(-7), "-7"},
		{"integral float", 42.0, "42.0"},
		{"fractional float", 1.5, "1.5"},
		{"json.Number", json.Number("1e3"), "1e3"},
		{"string", "12.5", "12.5"},
		{"bool", true, "null"},
		{"nil", nil, "null"},
		{"slice", []int{1}, "null"},
	}
	for _, tc := range cases {
		if got := numFromValue(tc.in).stringCompact(); got != tc.want {
			t.Errorf("numFromValue(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestStrdataFormatFloatAndRound(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{0, "0.0"},
		{42, "42.0"},
		{-2, "-2.0"},
		{1.5, "1.5"},
		{0.1, "0.1"},
		{-2.25, "-2.25"},
		{999999999999999, "999999999999999.0"},
		{1e15, "1e+15"},
		{1e16, "1e+16"},
		{-1e16, "-1e+16"},
	} {
		if got := formatFloat(tc.in); got != tc.want {
			t.Errorf("formatFloat(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		v      float64
		digits int
		want   float64
	}{
		{1.2345, 2, 1.23},
		{1.999, 2, 2},
		{0.05, 1, 0.1},
		{40.5, 0, 41},
		{-1.25, 1, -1.2},
	} {
		if got := roundFloat(tc.v, tc.digits); got != tc.want {
			t.Errorf("roundFloat(%v, %d) = %v, want %v", tc.v, tc.digits, got, tc.want)
		}
	}
}

func TestStrDataEncodeRoundTrip(t *testing.T) {
	plain := []byte(`{"k":"中文"}`)
	nonce := byte(0x2a)
	enc := EncodeStrData(plain, nonce)
	raw := encodingDecodeCustomB64(t, enc)
	if len(raw) != len(plain)+2 {
		t.Fatalf("decoded length = %d, want %d", len(raw), len(plain)+2)
	}
	if raw[0] != 0x41 || raw[1] != nonce {
		t.Fatalf("header = %#x %#x, want 0x41 %#x", raw[0], raw[1], nonce)
	}
	if got := string(RC4([]byte{nonce}, raw[2:])); got != string(plain) {
		t.Fatalf("RC4 round-trip = %q, want %q", got, string(plain))
	}

	// RC4 classic vector.
	sum := RC4([]byte("Key"), []byte("Plaintext"))
	if got := hex.EncodeToString(sum); got != "bbf316e8d940af0ad3" {
		t.Fatalf("RC4(Key, Plaintext) = %s, want bbf316e8d940af0ad3", got)
	}

	// Custom alphabet differs from standard base64 and pads normally.
	if got := b64CustomEncode([]byte("ABC")); got != "f6sp" {
		t.Fatalf(`b64CustomEncode("ABC") = %q, want f6sp`, got)
	}
	if got := b64CustomEncode(nil); got != "" {
		t.Fatalf("b64CustomEncode(nil) = %q, want empty", got)
	}
	if got := b64CustomEncode([]byte{1}); len(got) != 4 || !strings.HasSuffix(got, "==") {
		t.Fatalf("1-byte padding = %q", got)
	}
	if got := b64CustomEncode([]byte{1, 2}); len(got) != 4 || !strings.HasSuffix(got, "=") || strings.HasSuffix(got, "==") {
		t.Fatalf("2-byte padding = %q", got)
	}
	if got := b64CustomEncode([]byte{1, 2, 3}); len(got) != 4 || strings.Contains(got, "=") {
		t.Fatalf("3-byte padding = %q", got)
	}
}

func TestStrDataEnvelopeKeyOrder(t *testing.T) {
	want := `{"magic":538969122,"version":1,"dataType":8,"strData":"SD","tspFromClient":123,"ulr":0}`
	if got := mstokenEnvelope("SD", 123); got != want {
		t.Fatalf("mstokenEnvelope = %s\nwant %s", got, want)
	}
}

func TestStrDataReportBodyStructure(t *testing.T) {
	t.Setenv("DY_MSTOKEN_FIXED_NONCE", "170")
	t.Setenv("DY_MSTOKEN_FIXED_TIMESTAMP", "1700000000000")

	body, err := BuildMsTokenReportBody(6383, 1, "fixed-uuid", int64(42))
	if err != nil {
		t.Fatalf("BuildMsTokenReportBody: %v", err)
	}
	body2, err := BuildMsTokenReportBody(6383, 1, "fixed-uuid", int64(42))
	if err != nil {
		t.Fatalf("BuildMsTokenReportBody (second): %v", err)
	}
	if body != body2 {
		t.Fatalf("report body is not deterministic under fixed nonce/ts:\n%s\n%s", body, body2)
	}
	if strings.ContainsAny(body, " \n\t") {
		t.Fatalf("report body must be compact, got %q", body)
	}

	env, err := parseOrderedJSON([]byte(body))
	if err != nil {
		t.Fatalf("envelope is not valid JSON: %v", err)
	}
	if got, want := encodingJNodeKeys(env), []string{"magic", "version", "dataType", "strData", "tspFromClient", "ulr"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("envelope keys = %v, want %v", got, want)
	}
	for k, want := range map[string]string{
		"magic": "538969122", "version": "1", "dataType": "8",
		"tspFromClient": "1700000000000", "ulr": "0",
	} {
		if v, _ := env.get(k); v == nil || v.stringCompact() != want {
			t.Errorf("envelope[%s] = %v, want %s", k, v, want)
		}
	}

	sd, ok := env.get("strData")
	if !ok || sd.kind != 's' || sd.str == "" {
		t.Fatalf("strData node = %+v", sd)
	}
	raw := encodingDecodeCustomB64(t, sd.str)
	if len(raw) < 3 || raw[0] != 0x41 || raw[1] != 170 {
		t.Fatalf("strData framing = %#x, want 0x41 0xaa prefix", raw)
	}
	root, err := parseOrderedJSON(RC4([]byte{170}, raw[2:]))
	if err != nil {
		t.Fatalf("decrypted fingerprint is not valid JSON: %v", err)
	}
	if got, want := encodingJNodeKeys(root), []string{"nWID", "wID"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fingerprint keys = %v, want %v", got, want)
	}
	nwid, ok := root.get("nWID")
	if !ok || nwid.kind != 'o' || len(nwid.pairs) == 0 {
		t.Fatalf("nWID node = %+v", nwid)
	}
	wid, ok := root.get("wID")
	if !ok || wid.kind != 'o' {
		t.Fatalf("wID node = %+v", wid)
	}

	customRaw, ok := nwid.get("custom")
	if !ok || customRaw.kind != 's' {
		t.Fatalf("nWID.custom node = %+v", customRaw)
	}
	custom, err := parseOrderedJSON([]byte(customRaw.str))
	if err != nil {
		t.Fatalf("custom payload is not valid JSON: %v", err)
	}
	if got, want := encodingJNodeKeys(custom), []string{"version", "fxgDid", "uuid", "collectTime", "aid", "pageId"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("custom keys = %v, want %v", got, want)
	}
	for k, want := range map[string]string{
		"fxgDid": `""`, "uuid": `"fixed-uuid"`, "collectTime": "42", "aid": "6383", "pageId": "1",
	} {
		if v, _ := custom.get(k); v == nil || v.stringCompact() != want {
			t.Errorf("custom[%s] = %v, want %s", k, v, want)
		}
	}
	msVersion, ok := nwid.get("ms_version")
	if !ok || msVersion.kind != 's' {
		t.Fatalf("nWID.ms_version = %+v", msVersion)
	}
	if v, _ := custom.get("version"); v.str != msVersion.str {
		t.Fatalf("custom.version %q != ms_version %q", v.str, msVersion.str)
	}
}

// ---------------------------------------------------------------------------
// util.go
// ---------------------------------------------------------------------------

func TestUtilMD5HexAndIsAllDigits(t *testing.T) {
	for in, want := range map[string]string{
		"":    "d41d8cd98f00b204e9800998ecf8427e",
		"abc": "900150983cd24fb0d6963f7d28e17f72",
		"The quick brown fox jumps over the lazy dog": "9e107d9d372bb6826bd81d3542a419d6",
	} {
		if got := MD5Hex(in); got != want {
			t.Errorf("MD5Hex(%q) = %s, want %s", in, got, want)
		}
	}
	for in, want := range map[string]bool{
		"": true, "0": true, "007": true, "1234567890123456789": true,
		"12a": false, "-1": false, "1 2": false, "１２": false,
	} {
		if got := isAllDigits(in); got != want {
			t.Errorf("isAllDigits(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestUtilParseAwemeID(t *testing.T) {
	cases := []struct {
		in      string
		wantID  string
		wantErr bool
	}{
		{"1234567890123", "1234567890123", false},
		{" 42 ", "42", false},
		{"https://www.douyin.com/video/7333", "7333", false},
		{"https://www.douyin.com/video/7333?foo=bar", "7333", false},
		{"https://www.douyin.com/note/7333", "7333", false},
		{"https://www.douyin.com/slides/7333", "7333", false},
		{"https://www.douyin.com/?modal_id=7333", "7333", false},
		{"https://www.douyin.com/user/abc", "", true},
		{"https://www.douyin.com/?modal_id=abc", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		id, referer, err := ParseAwemeID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseAwemeID(%q) = (%q, %q), want error", tc.in, id, referer)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAwemeID(%q): %v", tc.in, err)
			continue
		}
		if id != tc.wantID {
			t.Errorf("ParseAwemeID(%q) id = %q, want %q", tc.in, id, tc.wantID)
		}
		if want := "https://www.douyin.com/video/" + tc.wantID; referer != want {
			t.Errorf("ParseAwemeID(%q) referer = %q, want %q", tc.in, referer, want)
		}
	}
}

func TestUtilGeneratorsShape(t *testing.T) {
	const msCharset = "ABCDEFGHIGKLMNOPQRSTUVWXYZabcdefghigklmnopqrstuvwxyz0123456789="
	ms := GenerateMsToken()
	if len(ms) != 107 {
		t.Fatalf("GenerateMsToken length = %d, want 107", len(ms))
	}
	for i := range len(ms) {
		if strings.IndexByte(msCharset, ms[i]) < 0 {
			t.Fatalf("GenerateMsToken produced %q outside its charset", ms[i])
		}
	}

	web := GenerateFakeWebID()
	if len(web) != 19 || !isAllDigits(web) {
		t.Fatalf("GenerateFakeWebID = %q, want 19 digits", web)
	}

	sv := GenerateSVWebID()
	parts := strings.Split(sv, "_")
	if len(parts) != 7 || parts[0] != "verify" {
		t.Fatalf("GenerateSVWebID = %q, want verify_<ts36>_<8>_<4>_<4>_<4>_<12>", sv)
	}
	if parts[1] == "" {
		t.Fatalf("GenerateSVWebID missing ts36 segment: %q", sv)
	}
	for _, c := range parts[1] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z') {
			t.Fatalf("GenerateSVWebID ts36 %q not base36", parts[1])
		}
	}
	for i, want := range []int{8, 4, 4, 4, 12} {
		if len(parts[2+i]) != want {
			t.Fatalf("GenerateSVWebID group %d = %q, want len %d", i, parts[2+i], want)
		}
	}
	if !strings.HasPrefix(parts[4], "4") {
		t.Fatalf("GenerateSVWebID version nibble = %q, want 4xxx", parts[4])
	}
	if !strings.Contains("89ab", parts[5][:1]) {
		t.Fatalf("GenerateSVWebID variant nibble = %q, want [89ab]xxx", parts[5])
	}

	t.Setenv("DY_FIXED_SV_WEB_ID", "verify_pinned")
	if got := GenerateSVWebID(); got != "verify_pinned" {
		t.Fatalf("DY_FIXED_SV_WEB_ID override = %q", got)
	}
	t.Setenv("DY_FIXED_SV_WEB_ID", "")
	t.Setenv("DY_FIXED_WEB_ID", "verify_fallback")
	if got := GenerateSVWebID(); got != "verify_fallback" {
		t.Fatalf("DY_FIXED_WEB_ID fallback = %q", got)
	}
}

func TestUtilCookiesParseAndSerialize(t *testing.T) {
	c := TransCookies(" a=1 ; b=2=3 ; bare ;  ; c= ; =d")
	if got, want := c.Names(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names = %v, want %v", got, want)
	}
	if v := c.Get("a"); v != "1" {
		t.Errorf("a = %q, want 1", v)
	}
	if v := c.Get("b"); v != "2=3" {
		t.Errorf("b = %q, want 2=3 (split on first =)", v)
	}
	if !c.Has("c") || c.Get("c") != "" {
		t.Errorf(`c = (%q, %v), want ("", true)`, c.Get("c"), c.Has("c"))
	}
	if c.Has("bare") {
		t.Error("bare token must not become a keyed cookie")
	}
	if c.Has("missing") {
		t.Error("Has(missing) = true")
	}
	if got, want := c.String(), "a=1; b=2=3; c=; bare"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}

	c.Set("b", "9") // replace in place
	if got, want := c.String(), "a=1; b=9; c=; bare"; got != want {
		t.Fatalf("String after Set = %q, want %q", got, want)
	}
	c.Del("a")
	c.Del("missing") // no-op
	if got, want := c.Names(), []string{"b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names after Del = %v, want %v", got, want)
	}

	c.MergeSetCookies(map[string]string{"d": "4", "e": ""})
	if got, want := c.Names(), []string{"b", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names after MergeSetCookies = %v, want %v (empty value skipped)", got, want)
	}

	if NewCookies().String() != "" {
		t.Fatal("empty Cookies String is non-empty")
	}
	if got := TransCookies(""); got.String() != "" || len(got.Names()) != 0 {
		t.Fatalf("TransCookies(\"\") = %q %v", got.String(), got.Names())
	}

	// A bare token that also has a value collapses to the bare spelling.
	dup := TransCookies("tok; tok=1")
	if got, want := dup.String(), "tok"; got != want {
		t.Fatalf("bare/keyed collision Serialize = %q, want %q", got, want)
	}
}
