package douyin

// 签名 / 派生逻辑的 hermetic 单元测试：SM3、a_bogus / X-Bogus、
// x-secsdk-web-signature、fpk1/fpk2、mstoken 纯函数、dtrait / bd-ticket 加密，
// 以及 challenge / fingerprint 的确定性变换。
//
// 所有期望值都来自独立实现（openssl dgst -sm3、openssl enc -P、md5(1)、
// Python RC4、Go 标准库 crypto/* 直接调用），而非被测代码本身。

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

// --- 测试专用小工具（全局唯一命名，避免与其他 *_test.go 冲突） -------------

const signCoreStdAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// signCoreCustomB64Decode 把自定义字母表的 base64 还原成字节，顺带断言
// 每个字符都属于字母表（或 '='）且 padding 合法。
func signCoreCustomB64Decode(t *testing.T, s, alphabet string) []byte {
	t.Helper()
	if len(alphabet) != 64 {
		t.Fatalf("字母表长度 %d, 期望 64", len(alphabet))
	}
	mapped := make([]byte, len(s))
	for i := range len(s) {
		c := s[i]
		if c == '=' {
			mapped[i] = '='
			continue
		}
		idx := strings.IndexByte(alphabet, c)
		if idx < 0 {
			t.Fatalf("字符 %q 不在字母表中", c)
		}
		mapped[i] = signCoreStdAlphabet[idx]
	}
	out, err := base64.StdEncoding.DecodeString(string(mapped))
	if err != nil {
		t.Fatalf("自定义 base64 无效: %v (%q)", err, s)
	}
	return out
}

func signCoreHeaderValue(h Headers, name string) (string, bool) {
	for _, kv := range h {
		if strings.EqualFold(kv.Name, name) {
			return kv.Value, true
		}
	}
	return "", false
}

func signCoreMustB64(t *testing.T, s string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64 解码失败: %v (%q)", err, s)
	}
	return out
}

// --- SM3 -------------------------------------------------------------------

func TestSM3PaddingBoundaries(t *testing.T) {
	// 期望值由 `openssl dgst -sm3` 独立算出。
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty", nil, "1ab21d8355cfa17f8e61194831e81a8f22bec8c728fefb747ed035eb5082aa2b"},
		{"55B", bytes.Repeat([]byte("a"), 55), "288337eef51eec62e7544d7270424c8dbe656254c99852870a73b2453a6a7fb1"},
		{"56B", bytes.Repeat([]byte("a"), 56), "ba00ebedaab54065a5fd4f9f56326016203166bcee3eed44ea868d59d67aa3c8"},
		{"57B", bytes.Repeat([]byte("a"), 57), "698e3fcc7a0b1515656a61db7e88805672285e83a4c24742dbade0c4010f32c0"},
		{"63B", bytes.Repeat([]byte("a"), 63), "587308543551881ebd70d27ad358ff5dcdf24ac54822e2f7b7c3edce0985d21b"},
		{"64B", bytes.Repeat([]byte("a"), 64), "616ec433c359e7c2b19f360e2b8f2a1b6e9ed76b8dc1a7d207b31a5341c611e9"},
		{"65B", bytes.Repeat([]byte("a"), 65), "3d1d94afa238ec3e2bbc20ad504702b24c16f2889c94973f2f8da3526c44e4bc"},
		{"200B", bytes.Repeat([]byte("abcd"), 50), "b100f9624d402126fdabb5afa22e8fae617d35e361f08b6cd0939506ba6a3be8"},
		{"hello world", []byte("hello world"), "44f0061e69fa6fdfc290c494654a05dc0c053da7e5c52b84ef93a9d67d3fff88"},
	}
	for _, tc := range cases {
		sum := sm3Hash(tc.in)
		if len(sum) != 32 {
			t.Fatalf("%s: sm3Hash 返回 %d 字节, 期望 32", tc.name, len(sum))
		}
		if got := hexSM3(tc.in); got != tc.want {
			t.Errorf("%s: SM3 = %s\n            want %s", tc.name, got, tc.want)
		}
		// 同一输入的重复调用必须一致（无全局状态）。
		if again := hexSM3(tc.in); again != tc.want {
			t.Errorf("%s: 第二次调用结果不稳定: %s", tc.name, again)
		}
	}
	// 55/56 字节跨越"填充恰好占满一个块"的边界，必须落在不同的填充分支。
	if hexSM3(bytes.Repeat([]byte("a"), 55)) == hexSM3(bytes.Repeat([]byte("a"), 56)) {
		t.Error("55B 与 56B 的摘要不应相同")
	}
	// 空输入与非空输入长度均为 32 字节。
	if len(sm3Hash([]byte{})) != len(sm3Hash([]byte("x"))) {
		t.Error("SM3 长度固定为 32 字节")
	}
}

// --- a_bogus ---------------------------------------------------------------

// signCoreCheckAbogusToken 校验 a_bogus 输出的字母表/padding/可解码性。
func signCoreCheckAbogusToken(t *testing.T, tok string) {
	t.Helper()
	if tok == "" {
		t.Fatal("a_bogus 为空")
	}
	if len(tok)%4 != 0 {
		t.Fatalf("a_bogus 长度 %d 不是 4 的倍数: %q", len(tok), tok)
	}
	pad := 0
	for i := len(tok) - 1; i >= 0 && tok[i] == '='; i-- {
		pad++
	}
	if pad > 2 {
		t.Fatalf("a_bogus padding 过多: %q", tok)
	}
	if strings.Contains(tok[:len(tok)-pad], "=") {
		t.Fatalf("a_bogus 的 '=' 只能出现在结尾: %q", tok)
	}
	if len(signCoreCustomB64Decode(t, tok, abogusAlphabetS4)) == 0 {
		t.Fatal("a_bogus 解码为空")
	}
}

func TestSignAbogusFixtureAlphabetAndHosts(t *testing.T) {
	t.Setenv("DY_ABOGUS_FIXED_RAND", "0.4142135623730951")
	t.Setenv("DY_ABOGUS_FIXED_NOW", "1720000000000")
	t.Setenv("DY_ABOGUS_FIXED_COUNTER", "2")
	t.Setenv("DY_ABOGUS_FIXED_RAND_SEQ", "")

	const query = "device_platform=webapp&aid=6383&aweme_id=7445533736877264178"
	s := NewAbogusSigner()

	first := s.SignQuery(query, "", "www.douyin.com")
	second := s.SignQuery(query, "", "www.douyin.com")
	if first != second {
		t.Fatalf("同一 fixture 下重复签名必须稳定:\n %s\n %s", first, second)
	}
	signCoreCheckAbogusToken(t, first)

	// host 参与签名（host 决定 sdkMinor / aid / pageID）。
	live := s.SignQuery(query, "", "live.douyin.com")
	if live == first {
		t.Fatal("不同 host 必须给出不同签名")
	}
	signCoreCheckAbogusToken(t, live)

	// Sign(url) 与 SignQuery(host, query) 等价。
	if got := s.Sign("https://live.douyin.com/7654321?"+query, ""); got != live {
		t.Errorf("Sign 未从 URL 还原出 live.douyin.com 的签名:\n got %s\nwant %s", got, live)
	}
	if got := s.Sign("https://www.douyin.com/?"+query, ""); got != first {
		t.Errorf("Sign 未从 URL 还原出 www.douyin.com 的签名:\n got %s\nwant %s", got, first)
	}

	// query / body 都必须进入摘要。
	if got := s.SignQuery(query+"&extra=1", "", "www.douyin.com"); got == first {
		t.Error("query 变化后签名必须变化")
	}
	if got := s.SignQuery(query, "body=1", "www.douyin.com"); got == first {
		t.Error("body 变化后签名必须变化")
	}

	// 字母表自身的不变量。
	if len(abogusAlphabetS4) != 64 {
		t.Fatalf("abogusAlphabetS4 长度 %d, 期望 64", len(abogusAlphabetS4))
	}
	seen := map[byte]bool{}
	for i := range len(abogusAlphabetS4) {
		if abogusAlphabetS4[i] == '=' {
			t.Fatal("abogusAlphabetS4 不应包含 '='")
		}
		if seen[abogusAlphabetS4[i]] {
			t.Fatalf("abogusAlphabetS4 存在重复字符 %q", abogusAlphabetS4[i])
		}
		seen[abogusAlphabetS4[i]] = true
	}
}

// 未固定随机数时必须仍然终止，且不同 signer 的随机流不同。
func TestAbogusUnfixedTokensDiffer(t *testing.T) {
	t.Setenv("DY_ABOGUS_FIXED_RAND", "")
	t.Setenv("DY_ABOGUS_FIXED_RAND_SEQ", "")
	t.Setenv("DY_ABOGUS_FIXED_NOW", "")
	t.Setenv("DY_ABOGUS_FIXED_COUNTER", "")

	const query = "device_platform=webapp&aid=6383"
	a := NewAbogusSigner().SignQuery(query, "", "www.douyin.com")
	b := NewAbogusSigner().SignQuery(query, "", "www.douyin.com")
	signCoreCheckAbogusToken(t, a)
	signCoreCheckAbogusToken(t, b)
	if a == b {
		t.Fatal("未固定随机数时两个 signer 不应给出同一 token")
	}
}

func TestAbogusHostParsing(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.douyin.com/aweme/v1/web/aweme/detail/?a=1", "www.douyin.com"},
		{"https://live.douyin.com/7654321", "live.douyin.com"},
		{"https://user:pw@www.douyin.com:443/a?b=1", "www.douyin.com"},
		{"http://www.douyin.com:8080/x", "www.douyin.com"},
		{"www.douyin.com/x?y=1", "www.douyin.com"},
		{"https://creator.douyin.com", "creator.douyin.com"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := hostOf(tc.in); got != tc.want {
			t.Errorf("hostOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAbogusPureHelperBoundaries(t *testing.T) {
	// counter 分级的下界（bucket 越大表示计数越小）。
	for _, tc := range []struct{ counter, want int }{
		{0, 6}, {1, 6}, {139, 6}, {140, 5}, {1283, 5}, {1284, 4}, {10745, 4}, {10746, 3}, {99999, 3},
	} {
		if got := z149CounterBucket(tc.counter); got != tc.want {
			t.Errorf("z149CounterBucket(%d) = %d, want %d", tc.counter, got, tc.want)
		}
	}

	// toInt32 必须按 32 位补码回绕。
	for _, tc := range []struct{ in, want int }{
		{0, 0}, {2147483647, 2147483647}, {2147483648, -2147483648},
		{4294967295, -1}, {4294967296, 0}, {4294967297, 1}, {-1, -1},
	} {
		if got := toInt32(tc.in); got != tc.want {
			t.Errorf("toInt32(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}

	// escapeDigest：命中保留字节时向后顺延；force 时直接返回保留值。
	digest := []byte{1, 2, 0, 3, 9}
	for _, tc := range []struct {
		name               string
		idx                int
		reserved, fallback byte
		force              bool
		want               byte
	}{
		{"直接命中", 0, 7, 5, false, 1},
		{"跳过保留字节", 2, 0, 5, false, 3},
		{"非保留字节", 2, 3, 5, false, 0},
		{"顺延一次仍非保留", 1, 2, 5, false, 0},
		{"越界用 fallback", 9, 7, 5, false, 5},
		{"force 返回 reserved", 0, 7, 5, true, 7},
	} {
		if got := escapeDigest(digest, tc.idx, tc.reserved, tc.fallback, tc.force); got != tc.want {
			t.Errorf("escapeDigest(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}

	// b64Custom 使用标准字母表时必须与 encoding/base64 完全一致。
	for n := 0; n <= 5; n++ {
		data := make([]byte, n)
		for i := range data {
			data[i] = byte(i*40 + 7)
		}
		if got, want := b64Custom(data, signCoreStdAlphabet), base64.StdEncoding.EncodeToString(data); got != want {
			t.Errorf("b64Custom(%d 字节, 标准字母表) = %q, want %q", n, got, want)
		}
	}

	// BrowserName / browserOffset 的边界。
	for _, tc := range []struct{ ua, name string }{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36", "Chrome"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0", "Firefox"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15", "Safari"},
		{"Mozilla/5.0 (Linux; Huawei; Android 12) AppleWebKit/537.36", "Huawei"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chromium/120.0.0.0", "Other"},
		{"", "Other"},
	} {
		if got := BrowserName(tc.ua); got != tc.name {
			t.Errorf("BrowserName(%.40q) = %q, want %q", tc.ua, got, tc.name)
		}
	}
	if got := z143RandOffset(0, "Firefox"); got != browserOffset("Firefox") {
		t.Errorf("z143RandOffset(0, Firefox) = %d, want %d", got, browserOffset("Firefox"))
	}
	for name, want := range map[string]int{"Chrome": 0, "Firefox": 40, "Safari": 81, "Edge": 125, "Huawei": 170, "Other": 210} {
		if got := browserOffset(name); got != want {
			t.Errorf("browserOffset(%s) = %d, want %d", name, got, want)
		}
	}
	// envFlagsByte：bit0 恒置位，node 加 bit2，Firefox 加 bit5。
	if got := envFlagsByte(false, "Chrome/151"); got != 1 {
		t.Errorf("envFlagsByte(desktop chrome) = %d, want 1", got)
	}
	if got := envFlagsByte(true, "Chrome/151"); got != 1|4 {
		t.Errorf("envFlagsByte(node) = %d, want %d", got, 1|4)
	}
	if got := envFlagsByte(false, "Firefox/128"); got != 1|32 {
		t.Errorf("envFlagsByte(firefox) = %d, want %d", got, 1|32)
	}
}

// --- X-Bogus ---------------------------------------------------------------

func signCoreXBogusRaw(roomID, userUniqueID string) string {
	return "live_id=1,aid=6383,version_code=180800,webcast_sdk_version=1.0.15," +
		"room_id=" + roomID + ",sub_room_id=,sub_channel_id=,did_rule=3," +
		"user_unique_id=" + userUniqueID + ",device_platform=web,device_type=," +
		"ac=,identity=audience"
}

func signCoreMD5Sum(b []byte) []byte {
	s := md5.Sum(b)
	return s[:]
}

// signCoreCheckXBogus 解密 X-Bogus 并校验字节级结构：
// eef/key 前缀、counter 字段、envFlags、摘要链（md5(md5(payload)) 与 md5(stub)）与校验和。
func signCoreCheckXBogus(t *testing.T, sig string, wantCounter byte, roomID, userUniqueID string) {
	t.Helper()
	inner := signCoreMD5Sum(nil) // payload 恒为空串
	h1 := signCoreMD5Sum(inner)
	h2 := signCoreMD5Sum(signCoreMD5Sum([]byte(signCoreXBogusRaw(roomID, userUniqueID))))

	raw := signCoreCustomB64Decode(t, sig, xbogusAlphabet)
	if len(raw) != 12 {
		t.Fatalf("counter=%d: X-Bogus 解码 %d 字节, 期望 12", wantCounter, len(raw))
	}
	if raw[0] != 0x40 && raw[0] != 0x50 {
		t.Fatalf("counter=%d: 首字节 %#x 不符合 (1<<6)|(n<<4)", wantCounter, raw[0])
	}
	keyByte := raw[1]
	plain := rc4Classic([]byte{keyByte}, raw[2:])
	if len(plain) != 10 {
		t.Fatalf("counter=%d: 明文 %d 字节, 期望 10", wantCounter, len(plain))
	}
	var chk byte
	for _, b := range plain[:9] {
		chk ^= b
	}
	if chk != plain[9] {
		t.Errorf("counter=%d: 校验字节不匹配: 计算 %#x, 实际 %#x", wantCounter, chk, plain[9])
	}
	if plain[0] != wantCounter || plain[1] != 0 {
		t.Errorf("counter=%d: 计数器字段 = [%d %d]", wantCounter, plain[0], plain[1])
	}
	if plain[2] != 1 {
		t.Errorf("counter=%d: envFlags = %#x, want 1", wantCounter, plain[2])
	}
	if plain[3] != 4|8 {
		t.Errorf("counter=%d: 常量字节 = %#x, want %#x", wantCounter, plain[3], 4|8)
	}
	if plain[4] != h1[14] || plain[5] != h1[15] || plain[6] != h2[14] || plain[7] != h2[15] {
		t.Errorf("counter=%d: 摘要片段不匹配: %v, want [%d %d %d %d]",
			wantCounter, plain[4:8], h1[14], h1[15], h2[14], h2[15])
	}
}

func TestSignXbogusWireShape(t *testing.T) {
	const roomID, userUniqueID = "7654321098765432100", "1234567890"
	s := NewXbogusSigner()
	first := s.Signature(roomID, userUniqueID)
	second := s.Signature(roomID, userUniqueID)
	if first == second {
		t.Fatal("signer 的 counter 每签名一次必须前进")
	}
	signCoreCheckXBogus(t, first, 1, roomID, userUniqueID)
	signCoreCheckXBogus(t, second, 2, roomID, userUniqueID)

	// counter 相同的前提下换输入：摘要链必须跟着新输入走。
	other := NewXbogusSigner().Signature(roomID+"9", userUniqueID)
	signCoreCheckXBogus(t, other, 1, roomID+"9", userUniqueID)
	if other == first {
		t.Error("room_id 变化必须改变 X-Bogus")
	}

	// xbEnvFlags 位语义。
	for _, tc := range []struct {
		firefox, topLevel, sane bool
		want                    byte
	}{
		{false, true, true, 1},
		{true, true, true, 1 | 32},
		{false, false, true, 1 | 64},
		{false, true, false, 1 | 128},
		{true, false, false, 1 | 32 | 64 | 128},
	} {
		if got := xbEnvFlags(tc.firefox, tc.topLevel, tc.sane); got != tc.want {
			t.Errorf("xbEnvFlags(%v,%v,%v) = %d, want %d", tc.firefox, tc.topLevel, tc.sane, got, tc.want)
		}
	}

	// rc4Classic 对抗已知 RC4 向量（Python 独立实现）。
	for _, tc := range []struct{ key, data, want string }{
		{"Key", "Plaintext", "bbf316e8d940af0ad3"},
		{"Wiki", "pedia", "1021bf0420"},
		{"Secret", "Attack at dawn", "45a01f645fc35b383552544b9bf5"},
	} {
		if got := hex.EncodeToString(rc4Classic([]byte(tc.key), []byte(tc.data))); got != tc.want {
			t.Errorf("rc4Classic(%q, %q) = %s, want %s", tc.key, tc.data, got, tc.want)
		}
	}
}

// --- x-secsdk-web-signature ------------------------------------------------

func TestSignSecsdkCanonicalQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"a=1&b=2", "a=1&b=2"},
		{"&a=1&&", "a=1"},
		{"k", "k="},
		{"c=x+y", "c=x%20y"},
		{"b=%2B", "b=%2B"},
		{"a%2Bb=1", "a+b=1"},
		{"a+b=1", "a b=1"},
		{"a=%zz", "a=%25zz"},
		{"v=!*'()~-._", "v=!*'()~-._"},
		{"kw=%E6%A6%B4%E8%8E%B2", "kw=%E6%A6%B4%E8%8E%B2"},
		{"raw=榴莲", "raw=%E6%A6%B4%E8%8E%B2"},
	}
	for _, tc := range cases {
		if got := CanonicalQuery(tc.in); got != tc.want {
			t.Errorf("CanonicalQuery(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if encodeComponent(tc.in) != quoteComponent(tc.in) {
			t.Errorf("encodeComponent 与 quoteComponent 必须一致: %q", tc.in)
		}
	}
	if got := quoteComponent(" "); got != "%20" {
		t.Errorf("quoteComponent(空格) = %q, want %%20", got)
	}

	for _, tc := range []struct{ in, base, query string }{
		{"https://x/y?timestamp=5&a=1&x-secsdk-web-signature=zz", "https://x/y", "a=1"},
		{"https://x/y?b=2", "https://x/y", "b=2"},
		{"https://x/y", "https://x/y", ""},
	} {
		base, query := splitSigned(tc.in)
		if base != tc.base || query != tc.query {
			t.Errorf("splitSigned(%q) = (%q, %q), want (%q, %q)", tc.in, base, query, tc.base, tc.query)
		}
	}
}

func TestSignSecsdkSensitivityAndIdempotence(t *testing.T) {
	const rawURL = "https://www.douyin.com/aweme/v1/web/aweme/post/?device_platform=webapp&aid=6383&sec_user_id=MS4wLjABAAAA"
	const ts = int64(1720000000)

	ts1, sig1, q1 := SignWeb(rawURL, ts, "UIFID-A")
	ts2, sig2, q2 := SignWeb(rawURL, ts, "UIFID-A")
	if ts1 != ts || ts1 != ts2 || sig1 != sig2 || q1 != q2 {
		t.Fatalf("固定输入必须给出确定签名: (%d,%s,%s) vs (%d,%s,%s)", ts1, sig1, q1, ts2, sig2, q2)
	}
	if len(sig1) != 32 {
		t.Fatalf("签名必须是 32 位十六进制, got %d", len(sig1))
	}
	if _, err := hex.DecodeString(sig1); err != nil {
		t.Fatalf("签名不是合法十六进制: %v", err)
	}
	if !strings.Contains(q1, "uifid=UIFID-A&timestamp=1720000000") {
		t.Errorf("签名查询串未包含 uifid/timestamp: %s", q1)
	}
	if strings.Contains(q1, "x-secsdk-web-signature") {
		t.Errorf("canonical query 不应包含签名参数: %s", q1)
	}

	if _, sig, _ := SignWeb(rawURL, ts+1, "UIFID-A"); sig == sig1 {
		t.Error("timestamp 变化必须改变签名")
	}
	if _, sig, _ := SignWeb(rawURL, ts, "UIFID-B"); sig == sig1 {
		t.Error("uifid 变化必须改变签名")
	}
	if _, sig, _ := SignWeb(rawURL+"&extra=1", ts, "UIFID-A"); sig == sig1 {
		t.Error("query 变化必须改变签名")
	}

	// URL 里已有 uifid 时以 URL 为准。
	withUifid := "https://www.douyin.com/aweme/v1/web/aweme/post/?device_platform=webapp&uifid=FIXED"
	_, _, q3 := SignWeb(withUifid, ts, "OTHER")
	if !strings.Contains(q3, "uifid=FIXED&timestamp=") || strings.Contains(q3, "OTHER") {
		t.Errorf("URL 内 uifid 应优先于参数: %s", q3)
	}

	// ts==0 → 取当前时间，并写回查询串。
	tsNow, _, qNow := SignWeb(rawURL, 0, "UIFID-A")
	if tsNow < 1700000000 {
		t.Fatalf("ts=0 应替换为当前秒级时间戳, got %d", tsNow)
	}
	if !strings.HasSuffix(qNow, "timestamp="+strconv.FormatInt(tsNow, 10)) {
		t.Errorf("查询串时间戳与返回值不一致: %s", qNow)
	}

	// 签署后的 URL 再次签署必须完全幂等。
	signed := SignWebURL(rawURL, ts, "UIFID-A")
	if !strings.HasSuffix(signed, "&x-secsdk-web-signature="+sig1) {
		t.Errorf("签名 URL 结尾不符: %s", signed)
	}
	if again := SignWebURL(signed, ts, "UIFID-A"); again != signed {
		t.Errorf("重复签署不幂等:\n got %s\nwant %s", again, signed)
	}
	if again := SignWebURL(signed, ts+1, "UIFID-A"); again == signed {
		t.Error("换时间戳重新签署应当给出不同 URL")
	}

	// path 不参与摘要（签名只覆盖 canonical query），同一 query 在不同 path 上一致。
	_, sigPathA, _ := SignWeb("https://www.douyin.com/a/?x=1", ts, "")
	_, sigPathB, _ := SignWeb("https://www.douyin.com/b/?x=1", ts, "")
	if sigPathA != sigPathB {
		t.Error("签名只覆盖 query，path 变化不应改变签名")
	}
}

// --- fpk1 / fpk2 -----------------------------------------------------------

func TestFpkComponentStringComposition(t *testing.T) {
	// 键排序、wrapper 解包、undefined、数字原样保留、键转义。
	profile := []byte(`{"fingerprintjs_components":{` +
		`"z":"last","a":"first","visitorId":"skip","id":"skip",` +
		`"w":{"value":7,"duration":3},"n":{"duration":9},` +
		`"num":1.50,"arr":[1,"x",true,null],"k:1":"v"}}`)
	got, err := FingerprintComponentString(profile)
	if err != nil {
		t.Fatalf("FingerprintComponentString: %v", err)
	}
	want := `a:"first"|arr:[1,"x",true,null]|k\:1:"v"|n:undefined|num:1.50|w:7|z:"last"`
	if got != want {
		t.Fatalf("组件串不匹配:\n got %s\nwant %s", got, want)
	}

	// 键顺序不同的等价档案必须给出同一组件串。
	shuffled := []byte(`{"components":{"z":"last","a":"first","visitorId":"skip","id":"skip",` +
		`"w":{"value":7,"duration":3},"n":{"duration":9},` +
		`"num":1.50,"arr":[1,"x",true,null],"k:1":"v"}}`)
	if got2, err := FingerprintComponentString(shuffled); err != nil || got2 != want {
		t.Errorf("components 回退键 + 不同键序应当等价: got %q err %v", got2, err)
	}

	// 单个组件变化必须改变摘要。
	changed := []byte(`{"fingerprintjs_components":{"a":"FIRST","w":{"value":7,"duration":3}}}`)
	other, err := FingerprintComponentString(changed)
	if err != nil {
		t.Fatal(err)
	}
	if MurmurX64Hash128(got, 0) == MurmurX64Hash128(other, 0) {
		t.Error("组件变化后 murmur 摘要必须变化")
	}
	if MurmurX64Hash128(got, 0) == MurmurX64Hash128(got, 1) {
		t.Error("seed 变化必须改变 murmur 摘要")
	}
	if len(MurmurX64Hash128(got, 0)) != 32 {
		t.Errorf("murmur 摘要必须是 32 位十六进制, got %q", MurmurX64Hash128(got, 0))
	}

	// 缺少组件字段时必须报错而不是返回空串。
	if _, err := FingerprintComponentString([]byte(`{"other":1}`)); err == nil {
		t.Error("缺少 fingerprintjs_components/components 应当报错")
	}
	if _, err := FingerprintComponentString([]byte(`{`)); err == nil {
		t.Error("非法 JSON 应当报错")
	}
}

func TestFpkDigestAndEncryption(t *testing.T) {
	digest, err := BuildFingerprintDigest()
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 32 {
		t.Fatalf("fingerprint digest 长度 %d, 期望 32", len(digest))
	}
	again, err := BuildFingerprintDigest()
	if err != nil || again != digest {
		t.Fatalf("同一档案的 digest 必须确定: %s vs %s (%v)", digest, again, err)
	}

	salt := []byte("SALTSALT")
	lower, err := BuildFpk1(digest, salt)
	if err != nil {
		t.Fatal(err)
	}
	if upper, err := BuildFpk1(strings.ToUpper(digest), salt); err != nil || upper != lower {
		t.Fatalf("大写摘要应归一化为小写: %v", err)
	}
	raw := signCoreMustB64(t, lower)
	if len(raw) < 16 || string(raw[:8]) != "Salted__" || !bytes.Equal(raw[8:16], salt) {
		t.Fatalf("fpk1 未按 OpenSSL Salted__ 格式封装: %q", lower)
	}
	if len(raw[16:])%16 != 0 {
		t.Fatalf("fpk1 密文 %d 字节, 不是 AES 块长整数倍", len(raw[16:]))
	}
	if len(raw[16:]) != 48 { // 32 字节明文 + 整块 PKCS#7 padding
		t.Fatalf("32 位摘要的密文应为 48 字节, got %d", len(raw[16:]))
	}
	if other, err := BuildFpk1(digest, []byte("OTHERSAL")); err != nil || other == lower {
		t.Fatalf("salt 变化必须改变密文: %v", err)
	}
}

// evpBytesToKey 与 `openssl enc -aes-256-cbc -md md5 -S ... -P` 对齐。
func TestFpkEvpBytesToKeyVectors(t *testing.T) {
	salt := []byte("12345678")
	cases := []struct {
		password, key, iv string
	}{
		{
			"byte_fingerprint",
			"4a62db8774c0f3d5acc06479178935ec9e3b3dcfb1bde0d9331bdb81dc03d08e",
			"4bec04a56a616bc6354ab88dbb3effc5",
		},
		{
			"p",
			"67b2cea5146593bd19364b439de60a348ee46def65c28afa0147d08502cb4628",
			"67d6262e1ec23af6c19a2ccb5a03e80d",
		},
	}
	for _, tc := range cases {
		key, iv := evpBytesToKey([]byte(tc.password), salt, 32, 16)
		if got := hex.EncodeToString(key); got != tc.key {
			t.Errorf("evpBytesToKey(%q) key = %s, want %s", tc.password, got, tc.key)
		}
		if got := hex.EncodeToString(iv); got != tc.iv {
			t.Errorf("evpBytesToKey(%q) iv = %s, want %s", tc.password, got, tc.iv)
		}
	}
	// password 为 "p" 时需要 3 轮 MD5（32+16=48 字节），最后一轮只取前 16 字节。
	if _, iv := evpBytesToKey([]byte("p"), salt, 32, 16); len(iv) != 16 {
		t.Fatalf("iv 长度 %d, 期望 16", len(iv))
	}
}

func TestFpk2MatchesMD5CLI(t *testing.T) {
	// 期望值由 `md5(1)` 独立算出。
	if got := BuildFpk2(defaultUA); got != "6967ec7261b3cbe6a91d798c6b951c60" {
		t.Errorf("fpk2(defaultUA) = %s", got)
	}
	if got := BuildFpk2("Mozilla/5.0 test-agent x"); got != "24484fee6cc82d9739fd648d23ef39f7" {
		t.Errorf("fpk2(test-agent) = %s", got)
	}
	if got := BuildFpk2("A"); len(got) != 32 || strings.ToLower(got) != got {
		t.Errorf("fpk2 应为 32 位小写 MD5, got %q", got)
	}
}

// --- mstoken 纯函数 ---------------------------------------------------------

func TestMsTokenHeadersAndExtraction(t *testing.T) {
	prof := GetProfile()

	full := mssdkHeaders(false)
	wantPairs := map[string]string{
		"sec-ch-ua-platform": prof.SecCHUAPlatform,
		"referer":            "https://www.douyin.com/",
		"user-agent":         prof.UA,
		"sec-ch-ua":          prof.SecCHUA,
		"content-type":       "text/plain;charset=UTF-8",
		"sec-ch-ua-mobile":   "?0",
		"accept":             "*/*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"cache-control":      "no-cache",
		"origin":             "https://www.douyin.com",
		"pragma":             "no-cache",
		"priority":           "u=1, i",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
	}
	if len(full) != len(wantPairs) {
		t.Fatalf("mssdkHeaders 有 %d 个头, 期望 %d", len(full), len(wantPairs))
	}
	for name, want := range wantPairs {
		if got, ok := signCoreHeaderValue(full, name); !ok || got != want {
			t.Errorf("mssdkHeaders[%s] = %q (存在 %v), want %q", name, got, ok, want)
		}
	}
	if _, ok := signCoreHeaderValue(full, "sec-fetch-storage-access"); ok {
		t.Error("storageAccess=false 不应带 sec-fetch-storage-access")
	}
	withStorage := mssdkHeaders(true)
	if got, ok := signCoreHeaderValue(withStorage, "sec-fetch-storage-access"); !ok || got != "active" {
		t.Errorf("storageAccess=true 必须带 sec-fetch-storage-access=active, got %q (存在 %v)", got, ok)
	}
	if len(withStorage) != len(wantPairs)+1 {
		t.Errorf("storageAccess=true 只应多一个头, got %d", len(withStorage))
	}

	const tok = "TOK-JAR-VALUE"
	long := strings.Repeat("A", 164) // 真实 token 量级（随机回退为 107）
	cases := []struct {
		name string
		resp *Response
		want string
	}{
		{"x-ms-token 优先", &Response{StatusCode: 200,
			Header:  fhttp.Header{"X-Ms-Token": {tok}},
			Cookies: []*fhttp.Cookie{ck("msToken", "COOKIE-VALUE")}}, tok},
		{"Set-Cookie 回退", &Response{StatusCode: 200,
			Header: fhttp.Header{"Set-Cookie": {"other=1; msToken=SET-COOKIE-TOK; Path=/"}}}, "SET-COOKIE-TOK"},
		{"cookie jar 回退", &Response{StatusCode: 200,
			Cookies: []*fhttp.Cookie{ck("a", "1"), ck("msToken", "JAR-TOK")}}, "JAR-TOK"},
		{"长 token 原样返回", &Response{StatusCode: 200,
			Cookies: []*fhttp.Cookie{ck("msToken", long)}}, long},
		{"无 token", &Response{StatusCode: 200}, ""},
		{"空 Set-Cookie 值不返回", &Response{StatusCode: 200,
			Header: fhttp.Header{"Set-Cookie": {"msToken=; Path=/"}}}, ""},
	}
	for _, tc := range cases {
		got := extractMsToken(tc.resp)
		if got != tc.want {
			t.Errorf("%s: extractMsToken = %q, want %q", tc.name, got, tc.want)
		}
	}

	fallback := GenerateMsToken()
	if len(fallback) != 107 {
		t.Errorf("随机 msToken 长度 %d, 期望 107", len(fallback))
	}
	for i := range len(fallback) {
		c := fallback[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '=') {
			t.Fatalf("随机 msToken 含非法字符 %q", c)
		}
	}
	if GenerateMsToken() == fallback {
		t.Error("两次随机 msToken 不应相同")
	}
}

func TestMsTokenPureHelpers(t *testing.T) {
	// queryEscapeStrict 只放行 unreserved 集合。
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"abcXYZ019-_.~", "abcXYZ019-_.~"},
		{"/", "%2F"},
		{"a=b", "a%3Db"},
		{"a b", "a%20b"},
		{"+/=", "%2B%2F%3D"},
		{"榴", "%E6%A6%B4"},
	} {
		if got := queryEscapeStrict(tc.in); got != tc.want {
			t.Errorf("queryEscapeStrict(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	// envInt：缺失或非数字 → 未设置；"0" 是合法值。
	t.Setenv("DY_T01SIGN_UNSET_INT", "")
	t.Setenv("DY_T01SIGN_ZERO_INT", "0")
	t.Setenv("DY_T01SIGN_BIG_INT", "604800")
	t.Setenv("DY_T01SIGN_BAD_INT", "12x")
	for _, tc := range []struct {
		key     string
		want    int
		present bool
	}{
		{"DY_T01SIGN_UNSET_INT", 0, false},
		{"DY_T01SIGN_ZERO_INT", 0, true},
		{"DY_T01SIGN_BIG_INT", 604800, true},
		{"DY_T01SIGN_BAD_INT", 0, false},
		{"DY_T01SIGN_ABSENT_INT", 0, false},
	} {
		got, ok := envInt(tc.key)
		if got != tc.want || ok != tc.present {
			t.Errorf("envInt(%s) = (%d,%v), want (%d,%v)", tc.key, got, ok, tc.want, tc.present)
		}
	}

	// 信封：键序（magic/version/dataType/strData/tspFromClient/ulr）与取值都是协议的一部分。
	env := mstokenEnvelope("AAA", 1720000000000)
	want := `{"magic":538969122,"version":1,"dataType":8,"strData":"AAA","tspFromClient":1720000000000,"ulr":0}`
	if env != want {
		t.Errorf("mstokenEnvelope =\n %s\nwant\n %s", env, want)
	}
	var decoded struct {
		Magic, Version, DataType int64
		StrData                  string `json:"strData"`
		TspFromClient            int64
		Ulr                      int64
	}
	if err := json.Unmarshal([]byte(env), &decoded); err != nil {
		t.Fatalf("信封不是合法 JSON: %v", err)
	}
	if decoded.Magic != 538969122 || decoded.Version != 1 || decoded.DataType != 8 ||
		decoded.StrData != "AAA" || decoded.TspFromClient != 1720000000000 || decoded.Ulr != 0 {
		t.Errorf("信封字段不符: %+v", decoded)
	}
	if len(decoded.StrData) == 0 {
		t.Error("strData 不应为空")
	}
	if empty := mstokenEnvelope("", 1); empty != `{"magic":538969122,"version":1,"dataType":8,"strData":"","tspFromClient":1,"ulr":0}` {
		t.Errorf("空 strData 信封 = %s", empty)
	}
}

// --- x-tt-session-dtrait ---------------------------------------------------

func TestSignDtraitHeaderStructure(t *testing.T) {
	pem, version, err := builtinTraitPubkey()
	if err != nil {
		t.Fatal(err)
	}
	if version != "d0" {
		t.Errorf("trait 版本 = %q, want d0", version)
	}
	if !strings.Contains(pem, "BEGIN RSA PUBLIC KEY") {
		t.Fatalf("内置公钥不是 PKCS#1 PEM: %.40q", pem)
	}
	pub, err := parseRSAPublicKey(pem)
	if err != nil {
		t.Fatalf("parseRSAPublicKey: %v", err)
	}
	if pub.N.BitLen() != 2048 || pub.E != 65537 {
		t.Errorf("公钥参数 = %d bit / e=%d, want 2048 / 65537", pub.N.BitLen(), pub.E)
	}
	for _, bad := range []string{
		"not a pem at all",
		"-----BEGIN RSA PUBLIC KEY-----\nAAECAw==\n-----END RSA PUBLIC KEY-----",
		"LS0tLS1CRUdJTiBSU0EgUFVCTElDIEtFWS0tLS0tCg==",
	} {
		if _, err := parseRSAPublicKey(bad); err == nil {
			t.Errorf("非法公钥应报错: %.30q", bad)
		}
	}

	// NIST AES-CBC 向量（首块由 FIPS 800-38A 给出）。
	key, _ := hex.DecodeString("2b7e151628aed2a6abf7158809cf4f3c")
	iv, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	pt, _ := hex.DecodeString("6bc1bee22e409f96e93d7e117393172a")
	out, err := aesCBCEncrypt(key, iv, pt)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 32 { // 16 字节明文 + 整块 PKCS#7 padding
		t.Fatalf("aesCBCEncrypt 16 字节输入 → %d 字节, 期望 32", len(out))
	}
	if got := hex.EncodeToString(out[:16]); got != "7649abac8119b246cee98e9b12e9197d" {
		t.Errorf("AES-CBC 首块 = %s", got)
	}
	for _, tc := range []struct{ size, want int }{
		{1, 16}, {15, 16}, {16, 32}, {17, 32}, {31, 32}, {32, 48},
	} {
		data := make([]byte, tc.size)
		copy(data, pt)
		got, err := aesCBCEncrypt(key, iv, data)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.want {
			t.Errorf("%d 字节输入 → %d 字节密文, 期望 %d", tc.size, len(got), tc.want)
		}
	}
	if same, err := aesCBCEncrypt(key, iv, pt); err != nil || !bytes.Equal(same, out) {
		t.Fatalf("相同输入的 aesCBCEncrypt 必须确定: %v", err)
	}
	if other, err := aesCBCEncrypt(key, []byte("0123456789abcdef"), pt); err != nil || bytes.Equal(other, out) {
		t.Fatalf("换 IV 必须改变密文: %v", err)
	}

	// 头部结构：d0_<b64(RSA(pk1, aes_key_hex))>_<b64(iv+cipher)>。
	c, _ := newStubClient(t, nil)
	c.DtraitBlob = `{"a":1,"b":"x"}`
	const path, origin, aid = "/aweme/v1/web/aweme/detail/", "https://www.douyin.com", 6383

	parts := strings.Split(mustDtraitHeader(t, c, path, aid, origin), "_")
	if len(parts) != 3 || parts[0] != "d0" {
		t.Fatalf("trait 头结构不符: %q", strings.Join(parts, "_"))
	}
	encKey := signCoreMustB64(t, parts[1])
	if len(encKey) != 256 { // 2048-bit RSA
		t.Fatalf("RSA 密文 %d 字节, 期望 256", len(encKey))
	}
	blob := signCoreMustB64(t, parts[2])
	if len(blob) < 32 || (len(blob)-16)%16 != 0 {
		t.Fatalf("AES 段长度非法: %d", len(blob))
	}

	// 第二次同 path/aid/origin：RSA 材料命中缓存，IV 必须新鲜。
	parts2 := strings.Split(mustDtraitHeader(t, c, path, aid, origin), "_")
	if parts2[1] != parts[1] {
		t.Error("同一 (aid,origin,blob) 应复用已缓存的 RSA 密钥材料")
	}
	if parts2[2] == parts[2] {
		t.Error("每次请求都应使用新的随机 IV")
	}
	// 换 path：材料仍复用（缓存键不含 path）。
	parts3 := strings.Split(mustDtraitHeader(t, c, "/other/path/", aid, origin), "_")
	if parts3[1] != parts[1] {
		t.Error("path 不应影响 RSA 材料缓存键")
	}
	// 换 origin：缓存键变化 → 新材料。
	parts4 := strings.Split(mustDtraitHeader(t, c, path, aid, "https://live.douyin.com"), "_")
	if parts4[1] == parts[1] {
		t.Error("不同 origin 必须使用新的 RSA 密钥材料")
	}
	// 换 blob（缓存键含 MD5(DtraitBlob)）。
	c.DtraitBlob = `{"a":2}`
	parts5 := strings.Split(mustDtraitHeader(t, c, path, aid, origin), "_")
	if parts5[1] == parts[1] {
		t.Error("DtraitBlob 变化必须使用新的 RSA 密钥材料")
	}
}

func mustDtraitHeader(t *testing.T, c *Client, path string, aid int, origin string) string {
	t.Helper()
	h, err := c.buildSessionDtrait(path, aid, origin)
	if err != nil {
		t.Fatalf("buildSessionDtrait(%s): %v", path, err)
	}
	return h
}

// --- bd-ticket-guard -------------------------------------------------------

const (
	signCoreBDPrvA = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	signCoreBDPrvB = "0fedcba0987654321fedcba0987654321fedcba0987654321fedcba098765432"
)

func TestBDTicketHKDFVectors(t *testing.T) {
	ikm := bytes.Repeat([]byte{0x0b}, 22)
	salt, _ := hex.DecodeString("000102030405060708090a0b0c")
	info, _ := hex.DecodeString("f0f1f2f3f4f5f6f7f8f9")

	// RFC 5869 Test Case 1 / 3。
	if got := hex.EncodeToString(hkdfSHA256(ikm, 42, salt, info)); got !=
		"3cb25f25faacd57a90434f64d0362f2a2d2d0a90cf1a5a4c5db02d56ecc4c5bf34007208d5b887185865" {
		t.Errorf("hkdf case1 = %s", got)
	}
	zeroSalt := hex.EncodeToString(hkdfSHA256(ikm, 42, nil, nil))
	if zeroSalt != "8da4e775a563c18f715f802a063c5a31b8a11f5c5ee1879ec3454e5f3c738d2d9d201395faa4b61a96c8" {
		t.Errorf("hkdf case3 = %s", zeroSalt)
	}
	if got := hex.EncodeToString(hkdfSHA256(ikm, 42, make([]byte, 32), nil)); got != zeroSalt {
		t.Error("空 salt 与 32 字节零 salt 必须等价（HKDF 默认 HashLen 个零）")
	}
	// 长度参数只截断 OKM，不改变前缀。
	if got := hex.EncodeToString(hkdfSHA256(ikm, 16, salt, info)); got != "3cb25f25faacd57a90434f64d0362f2a" {
		t.Errorf("hkdf L=16 = %s", got)
	}
}

func TestBDTicketECDHAndSigning(t *testing.T) {
	pubB := GenerateReeKey(signCoreBDPrvB)
	pubA := GenerateReeKey(signCoreBDPrvA)
	if pubB == "" || pubA == "" {
		t.Fatal("GenerateReeKey 对固定私钥不应失败")
	}
	for _, pub := range []string{pubA, pubB} {
		raw := signCoreMustB64(t, pub)
		if len(raw) != 65 || raw[0] != 0x04 {
			t.Fatalf("公钥应当是 0x04||X||Y 的 65 字节: %d 字节, 首字节 %#x", len(raw), raw[0])
		}
	}
	if got := GenerateReeKey("not-hex"); got != "" {
		t.Errorf("非法私钥应返回空串, got %q", got)
	}

	keyAB, err := DeriveECDHKey(signCoreBDPrvA, "pub."+pubB)
	if err != nil {
		t.Fatal(err)
	}
	keyBA, err := DeriveECDHKey(signCoreBDPrvB, "pub."+pubA)
	if err != nil {
		t.Fatal(err)
	}
	if len(keyAB) != 32 {
		t.Fatalf("ECDH 派生密钥长度 %d, 期望 32", len(keyAB))
	}
	if !bytes.Equal(keyAB, keyBA) {
		t.Fatal("ECDH 必须对称（双方派生出同一密钥）")
	}
	if got := hex.EncodeToString(keyAB); got != "a12d81c1dc024d0687351d5c01dd1728910f8d2bc6ab9c803a302ac3206af49a" {
		t.Errorf("派生的 HKDF 密钥 = %s", got)
	}
	if again, err := DeriveECDHKey(signCoreBDPrvA, "pub."+pubB); err != nil || !bytes.Equal(again, keyAB) {
		t.Fatalf("固定输入必须给出确定密钥: %v", err)
	}
	if _, err := DeriveECDHKey("zz", "pub."+pubB); err == nil {
		t.Error("非法私钥应报错")
	}
	if _, err := DeriveECDHKey(signCoreBDPrvA, "pub."+base64.StdEncoding.EncodeToString(make([]byte, 65))); err == nil {
		t.Error("非法服务端公钥点应报错")
	}
	if _, err := DeriveECDHKey(signCoreBDPrvA, "garbage"); err == nil {
		t.Error("非法服务端证书应报错")
	}

	// HMAC 分支：与 Python hmac-sha256 的独立常量比对。
	ecdhKey := make([]byte, 32)
	for i := range ecdhKey {
		ecdhKey[i] = byte(i)
	}
	const msg = "ticket=abc&path=/passport/ticket_guard/get_client_cert/&timestamp=1720000000"
	if got := GetReqSignHMAC(msg, ecdhKey); got != "iXCpBn/v3VazHkGdXLAYoXS/jNpmwLAjER+NN/pGbz8=" {
		t.Errorf("GetReqSignHMAC(msg) = %s", got)
	}
	if got := GetReqSignHMAC(map[string]any{"a": 1, "b": []int{2, 3}, "c": "x"}, ecdhKey); got !=
		"5kGtUg8bzxGuf0hshman+/nzMrDMHwh7MvEzrGDb/AU=" {
		t.Errorf("GetReqSignHMAC(map) = %s", got)
	}

	// ECDSA 分支：DER 签名必须能用公钥验证，且绑定具体消息。
	priv, err := parseRawP256(signCoreMustHex(t, signCoreBDPrvA))
	if err != nil {
		t.Fatal(err)
	}
	sigB64, err := GetReqSign(msg, signCoreBDPrvA)
	if err != nil {
		t.Fatal(err)
	}
	sig := signCoreMustB64(t, sigB64)
	sum := sha256.Sum256([]byte(msg))
	if !ecdsa.VerifyASN1(&priv.PublicKey, sum[:], sig) {
		t.Fatal("GetReqSign 的签名无法用对应公钥验证")
	}
	otherSum := sha256.Sum256([]byte(msg + "x"))
	if ecdsa.VerifyASN1(&priv.PublicKey, otherSum[:], sig) {
		t.Fatal("签名必须绑定到原始消息")
	}
	if _, err := GetReqSign(msg, "zz"); err == nil {
		t.Error("非法私钥应报错")
	}
}

func TestBDTicketClientDataAndVersion(t *testing.T) {
	const (
		api    = "/passport/ticket_guard/get_client_cert/"
		ticket = "abc"
		ts     = int64(1720000000)
	)
	resSign := "ticket=abc&path=/passport/ticket_guard/get_client_cert/&timestamp=1720000000"
	ecdhKey := make([]byte, 32)
	for i := range ecdhKey {
		ecdhKey[i] = byte(i)
	}

	cd, algo, err := GenerateBDTicketClientData(api, ticket, "ts.2.deadbeef", signCoreBDPrvA, ecdhKey, ts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if algo != "hmac" {
		t.Fatalf("有 ECDH 密钥时必须走 hmac 分支, got %q", algo)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(signCoreMustB64(t, cd), &payload); err != nil {
		t.Fatalf("client-data 不是合法 base64(JSON): %v", err)
	}
	if payload["ts_sign"] != "ts.2.deadbeef" || payload["req_content"] != "ticket,path,timestamp" {
		t.Errorf("payload = %v", payload)
	}
	if payload["timestamp"] != float64(ts) {
		t.Errorf("payload timestamp = %v, want %d", payload["timestamp"], ts)
	}
	if payload["req_sign"] != "iXCpBn/v3VazHkGdXLAYoXS/jNpmwLAjER+NN/pGbz8=" {
		t.Errorf("req_sign 未覆盖 resSign: %v", payload["req_sign"])
	}
	if _, ok := payload["t_trust"]; ok {
		t.Error("tTrust=nil 时不应出现 t_trust 字段")
	}

	trust := 7
	cd2, _, err := GenerateBDTicketClientData(api, ticket, "ts.2.deadbeef", signCoreBDPrvA, ecdhKey, ts, &trust)
	if err != nil {
		t.Fatal(err)
	}
	payload2 := map[string]any{}
	if err := json.Unmarshal(signCoreMustB64(t, cd2), &payload2); err != nil {
		t.Fatal(err)
	}
	if payload2["t_trust"] != float64(trust) {
		t.Errorf("t_trust = %v, want %d", payload2["t_trust"], trust)
	}
	if cd2 == cd {
		t.Error("t_trust 必须进入签名负载")
	}

	// 无 ECDH 密钥 → ecdsa 分支，签名覆盖同一个 resSign。
	cd3, algo3, err := GenerateBDTicketClientData(api, ticket, "ts.2.deadbeef", signCoreBDPrvA, nil, ts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if algo3 != "ecdsa" {
		t.Fatalf("无 ECDH 密钥时必须走 ecdsa 分支, got %q", algo3)
	}
	payload3 := map[string]any{}
	if err := json.Unmarshal(signCoreMustB64(t, cd3), &payload3); err != nil {
		t.Fatal(err)
	}
	priv, err := parseRawP256(signCoreMustHex(t, signCoreBDPrvA))
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(payload3["req_sign"].(string))
	if err != nil {
		t.Fatalf("req_sign 不是 base64: %v", err)
	}
	sum := sha256.Sum256([]byte(resSign))
	if !ecdsa.VerifyASN1(&priv.PublicKey, sum[:], sig) {
		t.Error("ecdsa 分支的 req_sign 必须覆盖 resSign")
	}

	// timestamp==0 → 取当前秒级时间戳。
	cd4, _, err := GenerateBDTicketClientData(api, ticket, "ts.2.deadbeef", signCoreBDPrvA, ecdhKey, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload4 := map[string]any{}
	if err := json.Unmarshal(signCoreMustB64(t, cd4), &payload4); err != nil {
		t.Fatal(err)
	}
	if got := payload4["timestamp"].(float64); got < 1700000000 {
		t.Errorf("timestamp=0 应替换为当前时间, got %v", got)
	}

	for _, tc := range []struct{ in, want string }{
		{"ts.1.abc", "1"}, {"ts.1", "1"}, {"ts.10", "1"},
		{"ts.2.abc", "2"}, {"ts.20", "2"}, {"", "2"},
	} {
		if got := TicketGuardVersion(tc.in); got != tc.want {
			t.Errorf("TicketGuardVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := jsonCompact(map[string]any{"b": 2, "a": 1}); got != `{"a":1,"b":2}` {
		t.Errorf("jsonCompact(map) = %s, 键必须排序", got)
	}
	if got := jsonCompact(struct {
		B int `json:"b"`
		A int `json:"a"`
	}{2, 1}); got != `{"b":2,"a":1}` {
		t.Errorf("jsonCompact(struct) 应保留字段顺序: %s", got)
	}
}

func signCoreMustHex(t *testing.T, s string) []byte {
	t.Helper()
	out, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex 解码失败: %v", err)
	}
	return out
}

// --- challenge -------------------------------------------------------------

func TestChallengeDeterministicTransform(t *testing.T) {
	// AES-256-CBC/PKCS#7，key=SHA-256(ua)，IV=key[16:] —— 期望值由 openssl 独立算出。
	ua := []byte("Mozilla/5.0 test-agent")
	for _, tc := range []struct{ pt, want string }{
		{"", "1b084cbb2e634ca813cb9e7d421c7860"},
		{"hello", "b777896e494f2550768db71d98e7ffd9"},
		{"0123456789abcdef", "6b50ac669536f6d9d0f84283fd5927c1234e2620f31ebfb24522e07130d18e57"},
	} {
		got, err := aesCBCPKCS7([]byte(tc.pt), ua)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(got) != tc.want {
			t.Errorf("aesCBCPKCS7(%q) = %s, want %s", tc.pt, hex.EncodeToString(got), tc.want)
		}
		if len(got)%16 != 0 {
			t.Errorf("密文长度 %d 不是块长整数倍", len(got))
		}
	}
	same, _ := aesCBCPKCS7([]byte("hello"), ua)
	if other, _ := aesCBCPKCS7([]byte("hello"), []byte("Mozilla/5.0 other")); bytes.Equal(other, same) {
		t.Error("UA 变化必须改变密文")
	}

	// escapeQuery：仅 unreserved + safe 通过，空格是 %20 而非 '+'。
	for _, tc := range []struct{ in, safe, want string }{
		{"", "-_.!~*'()", ""},
		{"a b", "-_.!~*'()", "a%20b"},
		{"a-b_c.d~e", "-_.!~*'()", "a-b_c.d~e"},
		{"a:b/c", "-_.!~*'()", "a%3Ab%2Fc"},
		{"榴", "-_.!~*'()", "%E6%A6%B4"},
		{"a b", "", "a%20b"},
		{"!*'()", "", "%21%2A%27%28%29"},
	} {
		if got := escapeQuery(tc.in, tc.safe); got != tc.want {
			t.Errorf("escapeQuery(%q, %q) = %q, want %q", tc.in, tc.safe, got, tc.want)
		}
	}

	// 档案加载：根节点稳定且含 fingerprint / fingerprintjs_components。
	root, err := loadChallengeProfile()
	if err != nil {
		t.Fatalf("loadChallengeProfile: %v", err)
	}
	if again, _ := loadChallengeProfile(); again != root {
		t.Error("档案只应解析一次（sync.Once）")
	}
	if _, ok := root.get("fingerprint"); !ok {
		t.Error("challenge_profile.json 缺少 fingerprint")
	}
	if _, ok := root.get("fingerprintjs_components"); !ok {
		t.Error("challenge_profile.json 缺少 fingerprintjs_components")
	}

	// sign：同一 UA 确定，换 UA 变化；输出是 URL-safe 的 base64。
	sign1, err := buildChallengeSign(nil, "UA-A")
	if err != nil {
		t.Fatal(err)
	}
	sign2, err := buildChallengeSign(nil, "UA-A")
	if err != nil || sign1 != sign2 {
		t.Fatalf("sign 必须确定: %v", err)
	}
	sign3, err := buildChallengeSign(nil, "UA-B")
	if err != nil || sign3 == sign1 {
		t.Fatalf("UA 变化必须改变 sign: %v", err)
	}
	if strings.ContainsAny(sign1, "+/") {
		t.Errorf("sign 必须是 URL-safe base64: %q", sign1)
	}
	if ct, err := base64.URLEncoding.DecodeString(sign1); err != nil || len(ct)%16 != 0 {
		t.Fatalf("sign 的 base64 载荷非法: %v (%d 字节)", err, len(ct))
	}

	// bit_env：passportiv 必填，且由 UA 派生。
	if _, err := BuildBitEnv("", "UA-A"); err == nil {
		t.Error("空 passportiv 必须报错")
	}
	be1, err := BuildBitEnv("IV-12345", "UA-A")
	if err != nil {
		t.Fatal(err)
	}
	if be2, _ := BuildBitEnv("IV-12345", "UA-A"); be2 != be1 {
		t.Error("bit_env 必须确定")
	}
	if be3, _ := BuildBitEnv("IV-12345", "UA-B"); be3 == be1 {
		t.Error("UA 变化必须改变 bit_env")
	}
	if ct, err := base64.URLEncoding.DecodeString(be1); err != nil || len(ct)%16 != 0 {
		t.Fatalf("bit_env 载荷非法: %v", err)
	}

	// sk：stack 确定 → 输出确定；PassportEncrypt 输出为小写十六进制。
	sk1, err := BuildChallengeSk("stack-A")
	if err != nil {
		t.Fatal(err)
	}
	if sk2, _ := BuildChallengeSk("stack-A"); sk2 != sk1 {
		t.Error("sk 必须确定")
	}
	if sk3, _ := BuildChallengeSk("stack-B"); sk3 == sk1 {
		t.Error("stack 变化必须改变 sk")
	}
	if sk1 == "" || strings.Trim(sk1, "0123456789abcdef") != "" {
		t.Errorf("sk 应为非空小写十六进制: %q", sk1)
	}

	// 表单体：先 sign 后 sk，确定，且 sign 段是 percent-encoded 的 base64url。
	body, err := BuildChallengeBody()
	if err != nil {
		t.Fatal(err)
	}
	if body2, _ := BuildChallengeBody(); body2 != body {
		t.Error("challenge body 必须确定")
	}
	if !strings.HasPrefix(body, "sign=") {
		t.Fatalf("表单体应以 sign= 开头: %.40q", body)
	}
	idx := strings.Index(body, "&sk=")
	if idx < 0 {
		t.Fatalf("表单体缺少 &sk=: %.40q", body)
	}
	signField, skField := body[len("sign="):idx], body[idx+len("&sk="):]
	if skField == "" || strings.Trim(skField, "0123456789abcdef") != "" {
		t.Errorf("sk 字段应为小写十六进制: %q", skField)
	}
	decoded, err := url.QueryUnescape(signField)
	if err != nil {
		t.Fatalf("sign 字段不是合法 percent-encoding: %v", err)
	}
	if ct, err := base64.URLEncoding.DecodeString(decoded); err != nil || len(ct)%16 != 0 {
		t.Fatalf("sign 字段载荷非法: %v", err)
	}
	wantSign, err := buildChallengeSign(nil, GetProfile().UA)
	if err != nil {
		t.Fatalf("buildChallengeSign(profile UA): %v", err)
	}
	if decoded != wantSign {
		t.Errorf("表单 sign 段应等于当前 UA 的 challenge sign: %q vs %q", decoded, wantSign)
	}
}

func TestChallengeDeviceProfile(t *testing.T) {
	p := GetProfile()
	dp := challengeDeviceProfile()
	if dp["ua"] != p.UA {
		t.Errorf("device profile ua = %v, want %v", dp["ua"], p.UA)
	}
	major := p.BrowserVersion
	if i := strings.IndexByte(major, '.'); i > 0 {
		major = major[:i]
	}
	if want, _ := strconv.Atoi(major); dp["browser_major"] != want {
		t.Errorf("browser_major = %v, want %d (来自 %q)", dp["browser_major"], want, p.BrowserVersion)
	}
	if dp["screen_width"] != mustAtoi(t, p.ScreenWidth) || dp["screen_height"] != mustAtoi(t, p.ScreenHeight) {
		t.Errorf("screen 尺寸映射错误: %v", dp)
	}
	for name, idx := range map[string]int{"inner_width": 0, "inner_height": 1, "outer_width": 2,
		"outer_height": 3, "avail_width": 4, "avail_height": 5} {
		if dp[name] != p.Geo[idx] {
			t.Errorf("%s = %v, want Geo[%d]=%d", name, dp[name], idx, p.Geo[idx])
		}
	}
	if got, ok := dp["languages"].([]string); !ok || len(got) != 5 || got[0] != "zh-CN" {
		t.Errorf("languages = %v", dp["languages"])
	}
	if dp["webgl_renderer"] != p.WebGLRenderer {
		t.Errorf("webgl_renderer = %v", dp["webgl_renderer"])
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("Atoi(%q): %v", s, err)
	}
	return n
}

// --- fingerprint profile ---------------------------------------------------

func signCoreResetProfile(t *testing.T) {
	t.Helper()
	profileCache = nil
	t.Cleanup(func() { profileCache = nil })
}

func TestFingerprintProfileDefaults(t *testing.T) {
	for _, k := range []string{
		"DY_FP_UA", "DY_FP_BROWSER_VERSION", "DY_FP_ENGINE_VERSION", "DY_FP_CPU_CORE_NUM",
		"DY_FP_DEVICE_MEMORY", "DY_FP_WEBGL_VENDOR", "DY_FP_WEBGL_RENDERER",
		"DY_FP_SCREEN_WIDTH", "DY_FP_SCREEN_HEIGHT", "DY_FP_INNER_WIDTH", "DY_FP_INNER_HEIGHT",
		"DY_FP_OUTER_WIDTH", "DY_FP_OUTER_HEIGHT", "DY_FP_AVAIL_WIDTH", "DY_FP_AVAIL_HEIGHT",
		"DY_FP_ACCEPT_LANGUAGE",
	} {
		t.Setenv(k, "")
	}
	// 非法整数回落默认值；合法整数照常生效。
	t.Setenv("DY_FP_SCREEN_X", "not-an-int")
	t.Setenv("DY_FP_SCREEN_Y", "7")
	signCoreResetProfile(t)

	p := GetProfile()
	if p.UA != defaultUA {
		t.Errorf("默认 UA = %q", p.UA)
	}
	if p.BrowserName != "Chrome" || p.EngineName != "Blink" || p.Platform != "Win32" {
		t.Errorf("固定字段错误: name=%q engine=%q platform=%q", p.BrowserName, p.EngineName, p.Platform)
	}
	if p.SecCHUAPlatform != `"Windows"` {
		t.Errorf("sec-ch-ua-platform = %q", p.SecCHUAPlatform)
	}
	if !strings.Contains(p.SecCHUA, `"Google Chrome";v="151"`) {
		t.Errorf("sec-ch-ua 未包含主版本: %q", p.SecCHUA)
	}
	if p.CpuCoreNum != "8" || p.DeviceMemory != "8" {
		t.Errorf("cpu/memory 默认值 = %q/%q", p.CpuCoreNum, p.DeviceMemory)
	}
	if p.ScreenWidth != "1920" || p.ScreenHeight != "1080" {
		t.Errorf("屏幕默认值 = %q/%q", p.ScreenWidth, p.ScreenHeight)
	}
	if p.AcceptLanguage != defaultAcceptLanguage {
		t.Errorf("accept-language = %q", p.AcceptLanguage)
	}
	if p.ScreenX != 0 || p.ScreenY != 7 {
		t.Errorf("ScreenX/ScreenY = %d/%d, want 0/7（非法值回落默认）", p.ScreenX, p.ScreenY)
	}
	want := [8]int{1920, 1080 - 225, 1920, 1080 - 48, 1920, 1080 - 48, 1920, 1080}
	if p.Geo != want {
		t.Errorf("Geo = %v, want %v", p.Geo, want)
	}
	if GetProfile() != p {
		t.Error("GetProfile 必须返回缓存的同一实例")
	}
}

func TestFingerprintProfileGeoOverrides(t *testing.T) {
	t.Setenv("DY_FP_UA", "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/128.0")
	t.Setenv("DY_FP_BROWSER_VERSION", "128.0")
	t.Setenv("DY_FP_ENGINE_VERSION", "128.0")
	t.Setenv("DY_FP_CPU_CORE_NUM", "4")
	t.Setenv("DY_FP_DEVICE_MEMORY", "8")
	t.Setenv("DY_FP_SCREEN_WIDTH", "1920")
	t.Setenv("DY_FP_SCREEN_HEIGHT", "1080")
	t.Setenv("DY_FP_INNER_WIDTH", "1900")
	t.Setenv("DY_FP_INNER_HEIGHT", "800")
	t.Setenv("DY_FP_OUTER_WIDTH", "1920")
	t.Setenv("DY_FP_OUTER_HEIGHT", "1032")
	t.Setenv("DY_FP_AVAIL_WIDTH", "1920")
	t.Setenv("DY_FP_AVAIL_HEIGHT", "1032")
	t.Setenv("DY_FP_SCREEN_X", "10")
	t.Setenv("DY_FP_SCREEN_Y", "20")
	signCoreResetProfile(t)

	p := GetProfile()
	if p.UA != "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/128.0" {
		t.Errorf("UA 覆盖失效: %q", p.UA)
	}
	if p.BrowserVersion != "128.0" || p.EngineVersion != "128.0" {
		t.Errorf("版本覆盖失效: %q/%q", p.BrowserVersion, p.EngineVersion)
	}
	if !strings.Contains(p.SecCHUA, `v="128"`) {
		t.Errorf("sec-ch-ua 未跟随 BROWSER_VERSION: %q", p.SecCHUA)
	}
	if p.CpuCoreNum != "4" || p.DeviceMemory != "8" {
		t.Errorf("cpu/memory 覆盖失效: %q/%q", p.CpuCoreNum, p.DeviceMemory)
	}
	if p.ScreenX != 10 || p.ScreenY != 20 {
		t.Errorf("ScreenX/ScreenY = %d/%d, want 10/20", p.ScreenX, p.ScreenY)
	}
	want := [8]int{1900, 800, 1920, 1032, 1920, 1032, 1920, 1080}
	if p.Geo != want {
		t.Errorf("Geo = %v, want %v", p.Geo, want)
	}
	// WEBGL_* / ACCEPT_LANGUAGE 未设置 → 仍取内置默认。
	if p.WebGLRenderer != defaultWebGLRenderer {
		t.Errorf("webgl_renderer 默认值被破坏: %q", p.WebGLRenderer)
	}
	if p.AcceptLanguage != defaultAcceptLanguage {
		t.Errorf("accept-language 默认值被破坏: %q", p.AcceptLanguage)
	}
}
