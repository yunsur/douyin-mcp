package douyin

// Hermetic unit tests for the creator publish session (creator.go /
// creator_media.go), the session helpers (api_session.go), the passport/login
// pure helpers (passport.go / login.go) and the Node resolution fallback
// (acrawler.go / node.go).
//
// No network, no external processes, no sleeps: every HTTP interaction goes
// through the shared stub transport, and the Node tests force DY_NODE="" with
// a PATH that contains neither node nor mise.

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// ---------------------------------------------------------------------------
// local helpers
// ---------------------------------------------------------------------------

func cuImageInfo(uri string, w, h int) map[string]any {
	return map[string]any{"uri": uri, "width": w, "height": h}
}

func cuJSONValue(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return v
}

func cuJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()
	v := cuJSONValue(t, raw)
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %T (%q)", v, raw)
	}
	return m
}

func cuJSONArray(t *testing.T, raw string) []any {
	t.Helper()
	v := cuJSONValue(t, raw)
	a, ok := v.([]any)
	if !ok {
		t.Fatalf("expected JSON array, got %T (%q)", v, raw)
	}
	return a
}

func cuQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return u.Query()
}

// cuB64Decode percent-decodes then base64-decodes a cookie payload.
func cuB64Decode(t *testing.T, raw string) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString(percentDecode(raw))
	if err != nil {
		t.Fatalf("base64 decode %q: %v", raw, err)
	}
	return out
}

// cuParseECPriv parses a SEC1 EC private PEM.
func cuParseECPriv(t *testing.T, privPEM string) *ecdsa.PrivateKey {
	t.Helper()
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		t.Fatalf("not PEM: %q", privPEM)
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse EC private key: %v", err)
	}
	return key
}

// cuBytesRepeat returns n copies of b.
func cuBytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// cuBootstrapped marks the login session as already bootstrapped so the QR
// tests exercise exactly one passport request without replaying the anonymous
// bootstrap chain.
func cuBootstrapped(c *Client) {
	st := c.loginState()
	st.mu.Lock()
	st.bootstrapped = true
	st.mu.Unlock()
}

// cuWriteExec drops an executable file at path.
func cuWriteExec(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// cuNoNode forces resolveNode() to the "no Node anywhere" branch: an explicit
// but empty DY_NODE plus a PATH holding no node and no mise.
func cuNoNode(t *testing.T) {
	t.Helper()
	t.Setenv("DY_NODE", "")
	t.Setenv("DOUYIN_NODE", "")
	t.Setenv("PATH", t.TempDir())
}

// ---------------------------------------------------------------------------
// creator.go — create_v2 item builders
// ---------------------------------------------------------------------------

func TestCreatorImageCreateItemDefaultsAndTagShaping(t *testing.T) {
	out, err := BuildImageCreateItem([]map[string]any{
		cuImageInfo("uri-1", 1080, 1920),
		cuImageInfo("uri-2", 720, 1280),
	}, PublishImageRequest{
		Title:      "T",
		Content:    "C",
		Tags:       []string{"a", "#b", "   "},
		CreationID: "cid-1",
	})
	if err != nil {
		t.Fatalf("BuildImageCreateItem: %v", err)
	}
	item, _ := out["item"].(map[string]any)
	if item == nil {
		t.Fatalf("missing item: %#v", out)
	}
	common, _ := item["common"].(map[string]any)
	if common == nil {
		t.Fatalf("missing common: %#v", item)
	}

	// tags are appended as "#tag" plain text; blank tags are dropped.
	if got := common["text"]; got != "T。C #a #b" {
		t.Fatalf("text = %q, want %q", got, "T。C #a #b")
	}
	// title marker (type 7) then the "。" separator marker (type 8).
	extra := cuJSONArray(t, common["text_extra"].(string))
	if len(extra) != 2 {
		t.Fatalf("text_extra len = %d, want 2 (%v)", len(extra), extra)
	}
	first, _ := extra[0].(map[string]any)
	second, _ := extra[1].(map[string]any)
	if first["start"] != float64(0) || first["end"] != float64(1) || first["type"] != float64(7) {
		t.Fatalf("title marker = %v", first)
	}
	if second["start"] != float64(1) || second["end"] != float64(2) || second["type"] != float64(8) {
		t.Fatalf("separator marker = %v", second)
	}

	// optional-field defaults: visibility 1 (private), download 1 (allowed),
	// timing -1, image media_type 2.
	if common["visibility_type"] != int64(1) {
		t.Fatalf("visibility_type = %v, want 1", common["visibility_type"])
	}
	if common["download"] != int64(1) {
		t.Fatalf("download = %v, want 1", common["download"])
	}
	if common["timing"] != int64(-1) {
		t.Fatalf("timing = %v, want -1", common["timing"])
	}
	if common["media_type"] != 2 {
		t.Fatalf("media_type = %v, want 2", common["media_type"])
	}
	if common["creation_id"] != "cid-1" {
		t.Fatalf("creation_id = %v", common["creation_id"])
	}
	for _, key := range []string{"challenges", "mentions", "activity"} {
		if got := common[key]; got != "[]" {
			t.Fatalf("%s = %v, want []", key, got)
		}
	}

	images, _ := common["images"].([]any)
	if len(images) != 2 {
		t.Fatalf("images = %v", common["images"])
	}
	img0, _ := images[0].(map[string]any)
	if img0["uri"] != "uri-1" || img0["width"] != int64(1080) || img0["height"] != int64(1920) {
		t.Fatalf("images[0] = %v", img0)
	}

	// no optional keys when the request did not ask for them
	for _, key := range []string{"mix_id", "poi_id", "poi_name", "hot_sentence"} {
		if _, ok := common[key]; ok {
			t.Fatalf("%s should be absent, got %v", key, common[key])
		}
	}
	anchor, _ := item["anchor"].(map[string]any)
	if len(anchor) != 0 {
		t.Fatalf("anchor = %v, want empty", anchor)
	}

	// cover defaults to the first image uri
	cover, _ := item["cover"].(map[string]any)
	if cover["poster"] != "uri-1" {
		t.Fatalf("cover poster = %v, want uri-1", cover["poster"])
	}
}

func TestCreatorImageCreateItemOptionalFieldsAndCoverSelection(t *testing.T) {
	no := false
	poi := map[string]any{"poi_id": "poi-7", "poi_name": "某地"}
	out, err := BuildImageCreateItem([]map[string]any{
		cuImageInfo("uri-1", 10, 20),
		cuImageInfo("uri-2", 30, 40),
	}, PublishImageRequest{
		Title:         "T",
		Content:       "C",
		Visibility:    "0",
		AllowDownload: &no,
		Timing:        7,
		CoverIndex:    5, // out of range: clamps to the last image
		MixID:         "mix-9",
		POI:           poi,
		HotSpot:       map[string]any{"word": "热点词"},
		CreationID:    "cid-2",
	})
	if err != nil {
		t.Fatalf("BuildImageCreateItem: %v", err)
	}
	item, _ := out["item"].(map[string]any)
	common, _ := item["common"].(map[string]any)
	if common["visibility_type"] != int64(0) {
		t.Fatalf("visibility_type = %v, want 0", common["visibility_type"])
	}
	if common["download"] != int64(0) {
		t.Fatalf("download = %v, want 0", common["download"])
	}
	if common["timing"] != int64(7) {
		t.Fatalf("timing = %v, want 7", common["timing"])
	}
	if common["mix_id"] != "mix-9" || common["poi_id"] != "poi-7" || common["poi_name"] != "某地" {
		t.Fatalf("optional fields = %v", common)
	}
	if common["hot_sentence"] != "热点词" {
		t.Fatalf("hot_sentence = %v", common["hot_sentence"])
	}
	cover, _ := item["cover"].(map[string]any)
	if cover["poster"] != "uri-2" {
		t.Fatalf("clamped cover poster = %v, want uri-2", cover["poster"])
	}
	anchor, _ := item["anchor"].(map[string]any)
	gotPOI, ok := anchor["poi"].(map[string]any)
	if !ok || gotPOI["poi_id"] != "poi-7" {
		t.Fatalf("anchor.poi = %v", anchor["poi"])
	}

	// an explicit cover uri always wins over CoverIndex.
	out2, err := BuildImageCreateItem([]map[string]any{cuImageInfo("uri-1", 10, 20)}, PublishImageRequest{
		CoverURI:   "uri-explicit",
		CreationID: "cid-3",
	})
	if err != nil {
		t.Fatalf("BuildImageCreateItem: %v", err)
	}
	item2, _ := out2["item"].(map[string]any)
	cover2, _ := item2["cover"].(map[string]any)
	if cover2["poster"] != "uri-explicit" {
		t.Fatalf("cover poster = %v, want uri-explicit", cover2["poster"])
	}
}

func TestCreatorImageCreateItemRejectsBadInput(t *testing.T) {
	if _, err := BuildImageCreateItem(nil, PublishImageRequest{}); err == nil {
		t.Fatal("empty image_infos must fail")
	}
	_, err := BuildImageCreateItem([]map[string]any{cuImageInfo("u", 1, 1)}, PublishImageRequest{Visibility: "all"})
	if err == nil || !strings.Contains(err.Error(), "visibility") {
		t.Fatalf("bad visibility error = %v", err)
	}

	for _, tc := range []struct {
		in   string
		want int64
		bad  bool
	}{
		{"", 1, false}, {"  ", 1, false}, {"0", 0, false}, {"1", 1, false},
		{"2", 2, false}, {" 2 ", 2, false}, {"3", 3, false}, {"x", 0, true},
	} {
		got, err := visibilityValue(tc.in)
		if tc.bad {
			if err == nil {
				t.Fatalf("visibilityValue(%q) must fail", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("visibilityValue(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}

func TestCreatorVideoCreateItemShape(t *testing.T) {
	out, err := BuildVideoCreateItem(
		map[string]any{"vid": "V1", "poster_uri": "poster-from-info"},
		PublishVideoRequest{
			Title:      "Ti",
			Content:    "De",
			Tags:       []string{"t1"},
			Visibility: "0",
			CoverDelay: 3,
			CreationID: "cid-v",
		},
		"cover-uri",
	)
	if err != nil {
		t.Fatalf("BuildVideoCreateItem: %v", err)
	}
	item, _ := out["item"].(map[string]any)
	common, _ := item["common"].(map[string]any)
	if common["text"] != "Ti De #t1" {
		t.Fatalf("text = %q", common["text"])
	}
	if common["item_title"] != "Ti" || common["caption"] != "De #t1" {
		t.Fatalf("item_title/caption = %v / %v", common["item_title"], common["caption"])
	}
	if common["media_type"] != 4 {
		t.Fatalf("media_type = %v, want 4", common["media_type"])
	}
	if common["video_id"] != "V1" {
		t.Fatalf("video_id = %v", common["video_id"])
	}
	// videoTiming passes 0 through (unlike imageTiming's -1 default).
	if common["timing"] != int64(0) {
		t.Fatalf("timing = %v, want 0", common["timing"])
	}
	if common["music_id"] != nil {
		t.Fatalf("music_id = %v, want nil", common["music_id"])
	}

	cover, _ := item["cover"].(map[string]any)
	if cover["poster"] != "cover-uri" {
		t.Fatalf("poster = %v", cover["poster"])
	}
	if cover["poster_delay"] != int64(3) {
		t.Fatalf("poster_delay = %v", cover["poster_delay"])
	}
	extend := cuJSONMap(t, cover["cover_tools_extend_info"].(string))
	coverInfo, _ := extend["coverInfo"].(map[string]any)
	if coverInfo["uri"] != "cover-uri" || coverInfo["firstFrameCoverUri"] != "cover-uri" {
		t.Fatalf("coverInfo = %v", coverInfo)
	}
	recommend, _ := extend["recommendCoverInfo"].(map[string]any)
	if recommend["isFromRecommend"] != false {
		t.Fatalf("recommendCoverInfo = %v", recommend)
	}
	if list, ok := extend["previewVideoList"].([]any); !ok || len(list) != 0 {
		t.Fatalf("previewVideoList = %v", extend["previewVideoList"])
	}

	chapter := cuJSONMap(t, item["chapter"].(map[string]any)["chapter"].(string))
	if chapter["chapter_type"] != float64(1) {
		t.Fatalf("chapter_type = %v", chapter["chapter_type"])
	}
	tools, _ := chapter["chapter_tools_info"].(map[string]any)
	if tools["is_pc"] != "1" || tools["is_syn"] != "1" {
		t.Fatalf("chapter_tools_info = %v", tools)
	}

	for _, key := range []string{"mix", "open_platform"} {
		if m, ok := item[key].(map[string]any); !ok || len(m) != 0 {
			t.Fatalf("%s = %v, want empty map", key, item[key])
		}
	}
	member, _ := item["selected_member"].(map[string]any)
	if member["is_selected_member_video"] != false {
		t.Fatalf("selected_member = %v", member)
	}
	sync, _ := item["sync"].(map[string]any)
	if sync["should_sync"] != false || sync["sync_to_toutiao"] != 0 {
		t.Fatalf("sync = %v", sync)
	}
	assistant, _ := item["assistant"].(map[string]any)
	if assistant["is_post_assistant"] != 1 || assistant["is_preview"] != 0 {
		t.Fatalf("assistant = %v", assistant)
	}

	// an empty posterURI falls back to the commit info's poster_uri.
	out2, err := BuildVideoCreateItem(map[string]any{"vid": "V2", "poster_uri": "p2"}, PublishVideoRequest{CreationID: "c2"}, "")
	if err != nil {
		t.Fatalf("BuildVideoCreateItem: %v", err)
	}
	item2, _ := out2["item"].(map[string]any)
	cover2, _ := item2["cover"].(map[string]any)
	if cover2["poster"] != "p2" {
		t.Fatalf("fallback poster = %v, want p2", cover2["poster"])
	}

	if _, err := BuildVideoCreateItem(nil, PublishVideoRequest{Visibility: "?"}, ""); err == nil {
		t.Fatal("bad visibility must fail")
	}
}

func TestCreatorJSLenUTF16Boundary(t *testing.T) {
	// "😀" is one rune but two UTF-16 code units; JS String.length semantics
	// drive the text_extra offsets.
	if got := creatorJSLen("😀"); got != 2 {
		t.Fatalf("creatorJSLen(😀) = %d, want 2", got)
	}
	text, extra := creatorBuildImageText("😀", "x")
	if text != "😀。x" {
		t.Fatalf("text = %q", text)
	}
	if len(extra) != 2 {
		t.Fatalf("extra = %v", extra)
	}
	second, _ := extra[1].(map[string]any)
	if second["start"] != 2 || second["end"] != 3 {
		t.Fatalf("separator marker = %v, want start 2 end 3", second)
	}
}

func TestCreatorAppendTagsAndTimingHelpers(t *testing.T) {
	for _, tc := range []struct {
		content string
		tags    []string
		want    string
	}{
		{"C", nil, "C"},
		{"C", []string{"a", "#b", " "}, "C #a #b"},
		{"", []string{"a", "#b"}, "#a #b"},
		{"  ", []string{"a"}, "#a"},
		{"C", []string{" ", ""}, "C"},
	} {
		if got := creatorAppendTags(tc.content, tc.tags); got != tc.want {
			t.Fatalf("creatorAppendTags(%q, %v) = %q, want %q", tc.content, tc.tags, got, tc.want)
		}
	}
	if got := imageTiming(0); got != -1 {
		t.Fatalf("imageTiming(0) = %d", got)
	}
	if got := imageTiming(12); got != 12 {
		t.Fatalf("imageTiming(12) = %d", got)
	}
	if got := videoTiming(0); got != 0 {
		t.Fatalf("videoTiming(0) = %d", got)
	}
	no := false
	yes := true
	if downloadValue(nil) != 1 || downloadValue(&yes) != 1 || downloadValue(&no) != 0 {
		t.Fatalf("downloadValue = %d/%d/%d", downloadValue(nil), downloadValue(&yes), downloadValue(&no))
	}
}

func TestCreatorCookieStrCreatorWireOrder(t *testing.T) {
	c, _ := newStubClient(t, nil)
	c.Cookie.Set("gd_random", "g1")
	c.Cookie.Set("x-web-secsdk-uid", "u1")
	c.Cookie.Set("csrf_session_id", "c1")
	got := c.CreatorCookieStr()
	want := []string{"gd_random=g1", "x-web-secsdk-uid=u1", "csrf_session_id=c1", "s_v_web_id="}
	at := -1
	for _, part := range want {
		idx := strings.Index(got, part)
		if idx < 0 {
			t.Fatalf("CreatorCookieStr %q is missing %q", got, part)
		}
		if idx < at {
			t.Fatalf("CreatorCookieStr %q has %q out of the browser priority order", got, part)
		}
		at = idx
	}
	// the shared main-domain session cookies follow the creator-only ones.
	if i, j := strings.Index(got, "sessionid=abc"), strings.Index(got, "gd_random=g1"); i < 0 || i < j {
		t.Fatalf("CreatorCookieStr %q must keep sessionid after the creator cookies", got)
	}
}

// ---------------------------------------------------------------------------
// creator.go — publish security gate
// ---------------------------------------------------------------------------

func TestCreatorPublishSecurityRequiresCredentials(t *testing.T) {
	c, st := newStubJSON(t, map[string]any{})
	if err := c.requirePublishSecurity(); err == nil ||
		!strings.Contains(err.Error(), "DY_TICKET") ||
		!strings.Contains(err.Error(), "DY_TS_SIGN") ||
		!strings.Contains(err.Error(), "DY_PRIVATE_KEY") {
		t.Fatalf("all-missing error = %v", err)
	}

	c.Ticket = "ticket-1"
	err := c.requirePublishSecurity()
	if err == nil || strings.Contains(err.Error(), "DY_TICKET") ||
		!strings.Contains(err.Error(), "DY_TS_SIGN") || !strings.Contains(err.Error(), "DY_PRIVATE_KEY") {
		t.Fatalf("partial error = %v", err)
	}

	c.TsSign = "tsign-1"
	c.PrivateKey = "priv-1"
	err = c.requirePublishSecurity()
	if err == nil || !strings.Contains(err.Error(), "DY_DTRAIT_BLOB") {
		t.Fatalf("dtrait error = %v", err)
	}

	// ts_sign must belong to the current cookie session when the cookie names it.
	c.Cookie.Set("bd_ticket_guard_ts_sign_id", "tsign-A")
	c.DtraitBlob = "blob"
	err = c.requirePublishSecurity()
	if err == nil || !strings.Contains(err.Error(), "同一次登录") {
		t.Fatalf("mismatch error = %v", err)
	}
	c.TsSign = "tsign-A.1"
	if err := c.requirePublishSecurity(); err != nil {
		t.Fatalf("matching session must pass: %v", err)
	}
	if !c.ticketMatchesSession() {
		t.Fatal("ticketMatchesSession = false for a prefix match")
	}
	c.TsSign = "other"
	if c.ticketMatchesSession() {
		t.Fatal("ticketMatchesSession = true for a mismatched ts_sign")
	}
	c.TsSign = ""
	if c.ticketMatchesSession() {
		t.Fatal("ticketMatchesSession = true without TsSign")
	}
	c.TsSign = "whatever"
	c.Cookie.Del("bd_ticket_guard_ts_sign_id")
	if !c.ticketMatchesSession() {
		t.Fatal("ticketMatchesSession = false without a session id cookie")
	}

	if reqs := st.requests(); len(reqs) != 0 {
		t.Fatalf("credential checks must not send requests, got %d", len(reqs))
	}
}

func TestCreatorCreateAwemeBlockedBeforeAnyRequest(t *testing.T) {
	c, st := newStubJSON(t, map[string]any{"status_code": 0, "item_id": "1"})
	_, err := c.createAweme(t.Context(), map[string]any{"item": map[string]any{}}, postImageReferer)
	if err == nil {
		t.Fatal("createAweme must fail without credentials")
	}
	for _, want := range []string{"create_v2 安全校验失败", "请求未发送", "DY_TICKET", "DY_TS_SIGN", "DY_PRIVATE_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q is missing %q", err, want)
		}
	}
	if reqs := st.requests(); len(reqs) != 0 {
		t.Fatalf("no request may be sent without credentials, got %d", len(reqs))
	}
}

func TestCreatorPublishContentValidatesBeforeUpload(t *testing.T) {
	c, st := newStubJSON(t, map[string]any{})
	c.Ticket = "ticket-1"
	c.TsSign = "tsign-1"
	c.PrivateKey = "priv-1"
	c.DtraitBlob = "blob"

	if _, err := c.PublishImageContent(t.Context(), PublishImageRequest{}); err == nil ||
		!strings.Contains(err.Error(), "images 不能为空") {
		t.Fatalf("empty images error = %v", err)
	}
	if _, err := c.PublishVideoContent(t.Context(), PublishVideoRequest{}); err == nil ||
		!strings.Contains(err.Error(), "video 不能为空") {
		t.Fatalf("empty video error = %v", err)
	}
	if reqs := st.requests(); len(reqs) != 0 {
		t.Fatalf("payload validation must precede uploads, got %d requests", len(reqs))
	}
}

// ---------------------------------------------------------------------------
// creator_media.go — AWS4-HMAC-SHA256 gateway signing
// ---------------------------------------------------------------------------

// cuFixedGatewayQuery is fed to creatorSignGateway in a scrambled insertion
// order so the signature also proves the canonical query is re-sorted.
func cuFixedGatewayQuery() *Params {
	q := NewParams()
	q.Add("user_id", "42")
	q.Add("s", "abc123")
	q.Add("app_id", "2906")
	q.Add("Version", "2018-08-01")
	q.Add("ServiceId", "jm8ajry58r")
	q.Add("Action", "ApplyImageUpload")
	return q
}

func TestAWS4GatewaySignatureIsDeterministic(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	sign := creatorSignGateway("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"SESSIONTOKEN", "GET", cuFixedGatewayQuery(), nil, "imagex", "cn-north-1", now)

	const wantAuth = "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20260830/cn-north-1/imagex/aws4_request, " +
		"SignedHeaders=x-amz-date;x-amz-security-token, " +
		"Signature=44fad7db658c66587280868d7430e9d6f5e604249dedede3b2bdb3024815d514"
	if got := sign["authorization"]; got != wantAuth {
		t.Fatalf("authorization =\n  %s\nwant\n  %s", got, wantAuth)
	}
	if sign["x-amz-date"] != "20260830T120000Z" {
		t.Fatalf("x-amz-date = %q", sign["x-amz-date"])
	}
	if sign["x-amz-security-token"] != "SESSIONTOKEN" {
		t.Fatalf("x-amz-security-token = %q", sign["x-amz-security-token"])
	}
	if _, ok := sign["x-amz-content-sha256"]; ok {
		t.Fatal("GET must not sign x-amz-content-sha256")
	}

	// same inputs, same signature (no clock or map-order dependence).
	again := creatorSignGateway("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"SESSIONTOKEN", "GET", cuFixedGatewayQuery(), nil, "imagex", "cn-north-1", now)
	if again["authorization"] != wantAuth {
		t.Fatalf("second call = %q", again["authorization"])
	}
}

func TestAWS4GatewayPostSignsPayloadHash(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	body := []byte(`{"SessionKey":"sk-1"}`)
	q := NewParams()
	q.Add("Action", "CommitImageUpload")
	q.Add("Version", "2018-08-01")
	q.Add("ServiceId", "jm8ajry58r")
	q.Add("app_id", "2906")
	q.Add("user_id", "42")

	sign := creatorSignGateway("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"SESSIONTOKEN", "POST", q, body, "imagex", "cn-north-1", now)

	sum := sha256.Sum256(body)
	wantHash := hex.EncodeToString(sum[:])
	if sign["x-amz-content-sha256"] != wantHash {
		t.Fatalf("x-amz-content-sha256 = %q, want %q", sign["x-amz-content-sha256"], wantHash)
	}
	const wantAuth = "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20260830/cn-north-1/imagex/aws4_request, " +
		"SignedHeaders=x-amz-content-sha256;x-amz-date;x-amz-security-token, " +
		"Signature=93d9484ab25aeb9b40185693cd9d95c6717ddc2e67603810ef42c4fe6caf0aa1"
	if got := sign["authorization"]; got != wantAuth {
		t.Fatalf("authorization =\n  %s\nwant\n  %s", got, wantAuth)
	}

	// a different body changes the signature.
	other := creatorSignGateway("AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		"SESSIONTOKEN", "POST", q, []byte(`{"SessionKey":"sk-2"}`), "imagex", "cn-north-1", now)
	if other["authorization"] == sign["authorization"] {
		t.Fatal("signature must depend on the payload hash")
	}
}

func TestCreatorAWSCanonicalQuerySortsAndQuotes(t *testing.T) {
	q := NewParams()
	q.Add("b", "2")
	q.Add("a b", "v/v")
	q.Add("a", "1")
	q.Add("empty", "")
	q.Add("til", "~-._")
	if got, want := creatorAWSCanonicalQuery(q), "a=1&a%20b=v%2Fv&b=2&empty=&til=~-._"; got != want {
		t.Fatalf("canonical query = %q, want %q", got, want)
	}
}

func TestCreatorGatewayHeadersCrossSiteLayout(t *testing.T) {
	q := NewParams().Add("Action", "CommitImageUpload")
	sign := creatorSignGateway("AK", "SK", "ST", "POST", q, []byte("{}"), "vod", "cn-north-1",
		time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC))
	h := creatorGatewayHeaders(sign, "application/json")
	for name, want := range map[string]string{
		"authorization":        sign["authorization"],
		"x-amz-date":           sign["x-amz-date"],
		"x-amz-security-token": "ST",
		"x-amz-content-sha256": sign["x-amz-content-sha256"],
		"content-type":         "application/json",
		"referer":              creatorOrigin + "/",
		"origin":               creatorOrigin,
		"sec-fetch-site":       "cross-site",
		"sec-fetch-mode":       "cors",
		"sec-fetch-dest":       "empty",
		"accept":               "*/*",
	} {
		if got, _ := h.Get(name); got != want {
			t.Fatalf("header %s = %q, want %q", name, got, want)
		}
	}
	if got, _ := h.Get("user-agent"); got == "" {
		t.Fatal("user-agent must be set")
	}
	plain := creatorGatewayHeaders(sign, "")
	if got, ok := plain.Get("content-type"); ok && got != "" {
		t.Fatalf("empty contentType must not set content-type, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// creator_media.go — ImageX / VOD gateway request layout
// ---------------------------------------------------------------------------

func TestCreatorImageXApplyUploadRequestLayout(t *testing.T) {
	c, st := newStubJSON(t, map[string]any{"Result": map[string]any{
		"UploadAddress": map[string]any{
			"StoreInfos":  []any{map[string]any{"StoreUri": "tos-cn-i-x/obj", "Auth": "auth-ticket", "UploadID": "upload-1"}},
			"UploadHosts": []any{"tos-cn-i-x.bytedance.com", "tos-cn-i-x.backup"},
			"SessionKey":  "sk-1",
		},
	}})
	sts := map[string]any{"AccessKeyID": "AK", "SecretAccessKey": "SK", "SessionToken": "ST"}

	node, err := c.applyImageUpload(t.Context(), sts, "u42", "randS")
	if err != nil {
		t.Fatalf("applyImageUpload: %v", err)
	}
	if node["store_uri"] != "tos-cn-i-x/obj" || node["auth"] != "auth-ticket" ||
		node["upload_id"] != "upload-1" || node["session_key"] != "sk-1" ||
		node["upload_host"] != "tos-cn-i-x.bytedance.com" {
		t.Fatalf("apply result = %#v", node)
	}

	req := st.last(t)
	if req.Method != fhttp.MethodGet {
		t.Fatalf("method = %s", req.Method)
	}
	if !strings.HasPrefix(req.URL, "https://"+imagexHost+"/?") {
		t.Fatalf("url = %s, want host %s at the root path", req.URL, imagexHost)
	}
	qv := cuQuery(t, req.URL)
	for key, want := range map[string]string{
		"Action":    "ApplyImageUpload",
		"Version":   "2018-08-01",
		"ServiceId": imagexServiceID,
		"app_id":    imagexAppID,
		"user_id":   "u42",
		"s":         "randS",
	} {
		if got := qv.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
	auth := req.Header("authorization")
	if !strings.HasPrefix(auth, awsAlgorithm+" Credential=AK/") ||
		!strings.Contains(auth, "/"+imagexRegion+"/"+imagexService+"/aws4_request") {
		t.Fatalf("authorization = %q", auth)
	}
	if req.Header("x-amz-security-token") != "ST" || req.Header("x-amz-date") == "" {
		t.Fatalf("gateway headers = %v", req.Headers)
	}
	if req.Cookie != "" {
		t.Fatalf("ImageX gateway requests are cookie-less, got %q", req.Cookie)
	}
}

func TestCreatorVODApplyUploadRequestLayout(t *testing.T) {
	c, st := newStubJSON(t, map[string]any{"Result": map[string]any{
		"InnerUploadAddress": map[string]any{
			"UploadNodes": []any{map[string]any{
				"StoreInfos":   []any{map[string]any{"StoreUri": "vid/obj", "Auth": "a", "UploadID": "i"}},
				"UploadHost":   "vod-upload.bytedance.com",
				"SessionKey":   "sk-v",
				"UploadHeader": map[string]any{"k": "v"},
			}},
		},
	}})
	sts := map[string]any{"AccessKeyID": "AK", "SecretAccessKey": "SK", "SessionToken": "ST"}

	node, err := c.applyVideoUpload(t.Context(), sts, 1048576, "u7")
	if err != nil {
		t.Fatalf("applyVideoUpload: %v", err)
	}
	if node["store_uri"] != "vid/obj" || node["upload_host"] != "vod-upload.bytedance.com" || node["session_key"] != "sk-v" {
		t.Fatalf("apply result = %#v", node)
	}

	req := st.last(t)
	if !strings.HasPrefix(req.URL, "https://"+vodHost+"/?") {
		t.Fatalf("url = %s, want host %s at the root path", req.URL, vodHost)
	}
	qv := cuQuery(t, req.URL)
	for key, want := range map[string]string{
		"Action":    "ApplyUploadInner",
		"Version":   vodAPIVersion,
		"SpaceName": vodSpaceName,
		"FileType":  "video",
		"IsInner":   "1",
		"FileSize":  "1048576",
		"app_id":    imagexAppID,
		"user_id":   "u7",
	} {
		if got := qv.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
	if qv.Get("s") == "" {
		t.Fatal("VOD apply must carry a random s parameter")
	}
	auth := req.Header("authorization")
	if !strings.Contains(auth, "/"+imagexRegion+"/"+vodService+"/aws4_request") {
		t.Fatalf("authorization = %q (want the vod service scope)", auth)
	}
}

func TestCreatorApplyUploadRejectsIncompleteResult(t *testing.T) {
	c, _ := newStubJSON(t, map[string]any{"Result": map[string]any{}})
	sts := map[string]any{"AccessKeyID": "AK", "SecretAccessKey": "SK", "SessionToken": "ST"}
	if _, err := c.applyImageUpload(t.Context(), sts, "", "s"); err == nil {
		t.Fatal("missing UploadAddress must fail")
	}
	if _, err := c.applyVideoUpload(t.Context(), sts, 1, ""); err == nil {
		t.Fatal("missing InnerUploadAddress must fail")
	}
}

// ---------------------------------------------------------------------------
// api_session.go — query encoding and id parsing
// ---------------------------------------------------------------------------

func TestSessionStandardEncodeQuery(t *testing.T) {
	p := NewParams()
	p.Add("q", "a b&c")
	p.Add("empty", "")
	p.Add("plus", "a+b")
	p.Add("u", "中文")
	if got, want := standardEncodeQuery(p), "q=a+b%26c&empty=&plus=a%2Bb&u=%E4%B8%AD%E6%96%87"; got != want {
		t.Fatalf("standardEncodeQuery = %q, want %q", got, want)
	}
	if got := standardEncodeQuery(NewParams()); got != "" {
		t.Fatalf("empty params = %q", got)
	}
	// unreserved bytes stay verbatim.
	q := NewParams().Add("k", "~-._Az09")
	if got, want := standardEncodeQuery(q), "k=~-._Az09"; got != want {
		t.Fatalf("unreserved = %q, want %q", got, want)
	}
}

func TestSessionToInt64EdgeInputs(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want int64
	}{
		{nil, 0},
		{float64(1.9), 1},
		{int64(9007199254740993), 9007199254740993},
		{int(77), 77},
		{json.Number("7000000000000000001"), 7000000000000000001},
		{json.Number("bad"), 0},
		{"123", 123},
		{"123abc", 123},
		{"abc", 0},
		{true, 0},
		{[]any{1}, 0},
	} {
		if got := toInt64(tc.in); got != tc.want {
			t.Fatalf("toInt64(%#v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSessionIDParsingFromResponses(t *testing.T) {
	t.Run("GetDeviceID", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{"id": "7311"})
		c.SetWebID("pinned")
		got, err := c.GetDeviceID(t.Context())
		if err != nil {
			t.Fatalf("GetDeviceID: %v", err)
		}
		if got != "7311" {
			t.Fatalf("device id = %q", got)
		}
		req := st.last(t)
		if req.Method != fhttp.MethodGet || !strings.HasPrefix(req.URL, douyinBase+"/aweme/v1/web/query/user?") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if qv := cuQuery(t, req.URL); qv.Get("webid") != "pinned" {
			t.Fatalf("webid = %q", qv.Get("webid"))
		}
	})

	t.Run("GetMyUID big number", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, `{"user_uid":7000000000000000001}`), nil
		})
		c.SetWebID("pinned")
		got, err := c.GetMyUID(t.Context())
		if err != nil {
			t.Fatalf("GetMyUID: %v", err)
		}
		if got != 7000000000000000001 {
			t.Fatalf("user_uid = %d (64-bit id lost?)", got)
		}
		if req := st.last(t); !strings.Contains(req.URL, "/aweme/v1/web/query/user/") {
			t.Fatalf("url = %s", req.URL)
		}
	})

	t.Run("GetMyUID string number", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"user_uid": "42"})
		c.SetWebID("pinned")
		got, err := c.GetMyUID(t.Context())
		if err != nil || got != 42 {
			t.Fatalf("GetMyUID = %d, %v", got, err)
		}
	})

	t.Run("GetMySecUID creator API first", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{"user": map[string]any{"sec_uid": "MS4wLjABAAAA"}})
		got, err := c.GetMySecUID(t.Context())
		if err != nil {
			t.Fatalf("GetMySecUID: %v", err)
		}
		if got != "MS4wLjABAAAA" {
			t.Fatalf("sec_uid = %q", got)
		}
		reqs := st.requests()
		if len(reqs) != 1 || !strings.HasPrefix(reqs[0].URL, creatorBase+"/web/api/media/user/info/") {
			t.Fatalf("requests = %#v", reqs)
		}
		if !strings.Contains(reqs[0].Cookie, "sessionid=abc") {
			t.Fatalf("cookie header = %q", reqs[0].Cookie)
		}
	})

	t.Run("GetMySecUID HTML fallback", func(t *testing.T) {
		c, st := newStubClient(t, func(req stubRequest) (*Response, error) {
			if strings.Contains(req.URL, "/web/api/media/user/info/") {
				return statusResponse(200, `{"user":{}}`), nil
			}
			return statusResponse(200, `{"data":{\"secUid\":\"SEC-FROM-HTML\"}}`), nil
		})
		got, err := c.GetMySecUID(t.Context())
		if err != nil {
			t.Fatalf("GetMySecUID: %v", err)
		}
		if got != "SEC-FROM-HTML" {
			t.Fatalf("sec_uid = %q", got)
		}
		reqs := st.requests()
		if len(reqs) != 2 || !strings.Contains(reqs[1].URL, "/user/self?") {
			t.Fatalf("fallback requests = %#v", reqs)
		}
		if qv := cuQuery(t, reqs[1].URL); qv.Get("from_tab_name") != "main" {
			t.Fatalf("from_tab_name = %q", qv.Get("from_tab_name"))
		}
	})

	t.Run("GetMySecUID failure", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"user": map[string]any{}})
		if _, err := c.GetMySecUID(t.Context()); err == nil {
			t.Fatal("both sources empty must fail")
		}
	})

	t.Run("HTMLUserUniqueID", func(t *testing.T) {
		body := `window.__INIT__={\"user_unique_id\":\"999888\"}`
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, body), nil
		})
		if got := c.HTMLUserUniqueID(t.Context(), douyinBase+"/user/self"); got != "999888" {
			t.Fatalf("user_unique_id = %q", got)
		}
		req := st.last(t)
		if !strings.Contains(req.Cookie, "sessionid=abc") {
			t.Fatalf("cookie header = %q", req.Cookie)
		}
		missing, _ := newStubJSON(t, map[string]any{})
		if got := missing.HTMLUserUniqueID(t.Context(), douyinBase+"/user/self"); got != "" {
			t.Fatalf("no marker must return empty, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// passport.go — ticket-guard / passport pure helpers
// ---------------------------------------------------------------------------

func TestPassportEncryptFixedVectors(t *testing.T) {
	// UTF-8 bytes XOR 5, lowercase hex, NO zero padding (JS toString(16)).
	for _, tc := range []struct{ in, want string }{
		{"abc", "646766"},
		{"\x0b", "e"}, // 0x0b ^ 5 == 0x0e -> single hex digit
		{"", ""},
	} {
		if got := PassportEncrypt(tc.in); got != tc.want {
			t.Fatalf("PassportEncrypt(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// multi-byte UTF-8 encodes byte-wise (中 == E4 B8 AD, each byte XOR 5).
	if got := PassportEncrypt("中"); got != "e1bda8" {
		t.Fatalf("PassportEncrypt(中) = %q, want e1bda8", got)
	}
}

func TestPassportGenerateECKeypairShape(t *testing.T) {
	privPEM, pubPEM, err := GenerateECKeypair()
	if err != nil {
		t.Fatalf("GenerateECKeypair: %v", err)
	}
	privBlock, _ := pem.Decode([]byte(privPEM))
	if privBlock == nil || privBlock.Type != "EC PRIVATE KEY" {
		t.Fatalf("private PEM = %q", privPEM)
	}
	pubBlock, _ := pem.Decode([]byte(pubPEM))
	if pubBlock == nil || pubBlock.Type != "PUBLIC KEY" {
		t.Fatalf("public PEM = %q", pubPEM)
	}
	priv, err := x509.ParseECPrivateKey(privBlock.Bytes)
	if err != nil {
		t.Fatalf("parse EC private key: %v", err)
	}
	pubAny, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("parse PKIX public key: %v", err)
	}
	pub, ok := pubAny.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("public key type = %T", pubAny)
	}
	if priv.Curve != elliptic.P256() || pub.Curve != elliptic.P256() {
		t.Fatal("keypair must use P-256")
	}
	if priv.PublicKey.X.Cmp(pub.X) != 0 || priv.PublicKey.Y.Cmp(pub.Y) != 0 {
		t.Fatal("public PEM does not match the private key")
	}

	priv2, _, err := GenerateECKeypair()
	if err != nil {
		t.Fatalf("second GenerateECKeypair: %v", err)
	}
	if priv2 == privPEM {
		t.Fatal("two generated keypairs must differ")
	}

	// the ree public key is the 65-byte uncompressed point.
	ree := GenerateReeKey(privPEM)
	raw, err := base64.StdEncoding.DecodeString(ree)
	if err != nil {
		t.Fatalf("ree key base64: %v", err)
	}
	if len(raw) != 65 || raw[0] != 0x04 {
		t.Fatalf("ree key = %d bytes starting with %#x", len(raw), raw[0])
	}
}

func TestPassportClientDataCookieShape(t *testing.T) {
	privPEM, _, err := GenerateECKeypair()
	if err != nil {
		t.Fatalf("GenerateECKeypair: %v", err)
	}
	cookie := BuildClientDataCookie(privPEM)
	if strings.ContainsAny(cookie, "+/=") {
		t.Fatalf("cookie must be percent-encoded base64, got %q", cookie)
	}
	payload := cuJSONMap(t, string(cuB64Decode(t, cookie)))
	if payload["bd-ticket-guard-version"] != float64(2) {
		t.Fatalf("bd-ticket-guard-version = %v", payload["bd-ticket-guard-version"])
	}
	if payload["bd-ticket-guard-iteration-version"] != float64(1) {
		t.Fatalf("iteration-version = %v", payload["bd-ticket-guard-iteration-version"])
	}
	if payload["bd-ticket-guard-web-version"] != float64(2) {
		t.Fatalf("web-version = %v", payload["bd-ticket-guard-web-version"])
	}
	ree, _ := payload["bd-ticket-guard-ree-public-key"].(string)
	if raw, err := base64.StdEncoding.DecodeString(ree); err != nil || len(raw) != 65 {
		t.Fatalf("ree public key = %q (%v)", ree, err)
	}
}

func TestPassportClientDataV2CookieFixedPayload(t *testing.T) {
	privPEM, _, err := GenerateECKeypair()
	if err != nil {
		t.Fatalf("GenerateECKeypair: %v", err)
	}
	priv := cuParseECPriv(t, privPEM)

	// A deterministic server public key: derived from a fixed 32-byte scalar,
	// published in the "pub.<base64 point>" wire form.
	serverPriv, err := ecdh.P256().NewPrivateKey(cuBytesRepeat(0x2b, 32))
	if err != nil {
		t.Fatalf("server key: %v", err)
	}
	serverCert := "pub." + base64.StdEncoding.EncodeToString(serverPriv.PublicKey().Bytes())

	const secTS = "1700000000"
	const tsSign = "ts.1-fixed"
	cookie, err := BuildClientDataV2Cookie(privPEM, secTS, serverCert, tsSign)
	if err != nil {
		t.Fatalf("BuildClientDataV2Cookie: %v", err)
	}
	again, err := BuildClientDataV2Cookie(privPEM, secTS, serverCert, tsSign)
	if err != nil {
		t.Fatalf("BuildClientDataV2Cookie (2nd): %v", err)
	}
	if cookie != again {
		t.Fatal("v2 client data must be deterministic for fixed inputs")
	}

	payload := cuJSONMap(t, string(cuB64Decode(t, cookie)))
	if payload["req_content"] != "sec_ts" || payload["sec_ts"] != secTS || payload["ts_sign"] != tsSign {
		t.Fatalf("payload = %v", payload)
	}

	// req_sign = base64(HMAC-SHA256(HKDF(ECDH(priv, serverPub)), "sec_ts="+secTS)).
	sharedPriv, err := ecdh.P256().NewPrivateKey(priv.D.FillBytes(make([]byte, 32)))
	if err != nil {
		t.Fatalf("client ecdh key: %v", err)
	}
	shared, err := sharedPriv.ECDH(serverPriv.PublicKey())
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	key, err := hkdf.Key(sha256.New, shared, nil, "", 32)
	if err != nil {
		t.Fatalf("hkdf: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("sec_ts=" + secTS))
	if want := base64.StdEncoding.EncodeToString(mac.Sum(nil)); payload["req_sign"] != want {
		t.Fatalf("req_sign = %v, want %v", payload["req_sign"], want)
	}

	// changing the payload inputs changes the signature.
	other, err := BuildClientDataV2Cookie(privPEM, "1700000001", serverCert, tsSign)
	if err != nil {
		t.Fatalf("BuildClientDataV2Cookie (other ts): %v", err)
	}
	if other == cookie {
		t.Fatal("sec_ts must affect the v2 client data")
	}

	// a malformed server cert is rejected instead of silently signing garbage.
	if _, err := BuildClientDataV2Cookie(privPEM, secTS, "not-a-cert", tsSign); err == nil {
		t.Fatal("bad server cert must fail")
	}
}

func TestPassportParseTicketGuardServerData(t *testing.T) {
	payload := `{"ticket":"TICKET-1","ts_sign":"ts.1abc","client_cert":"CERT-1","create_time":1700000000,"log_id":"LOG-1"}`
	encoded := percentEncode(base64.StdEncoding.EncodeToString([]byte(payload)))

	t.Run("header", func(t *testing.T) {
		resp := &Response{StatusCode: 200, Header: fhttp.Header{}}
		resp.Header.Set("bd-ticket-guard-server-data", encoded)
		info := ParseTicketGuardServerData(resp)
		if info == nil {
			t.Fatal("header payload not parsed")
		}
		if info.Ticket != "TICKET-1" || info.TsSign != "ts.1abc" || info.ClientCert != "CERT-1" || info.LogID != "LOG-1" {
			t.Fatalf("info = %#v", info)
		}
	})

	t.Run("cookie fallback", func(t *testing.T) {
		resp := &Response{StatusCode: 200, Cookies: []*fhttp.Cookie{ck("bd_ticket_guard_server_data", encoded)}}
		info := ParseTicketGuardServerData(resp)
		if info == nil || info.Ticket != "TICKET-1" {
			t.Fatalf("cookie payload = %#v", info)
		}
	})

	t.Run("invalid payloads", func(t *testing.T) {
		for name, resp := range map[string]*Response{
			"absent":     {StatusCode: 200},
			"not base64": {StatusCode: 200, Header: fhttp.Header{"Bd-Ticket-Guard-Server-Data": []string{"###not-base64###"}}},
			"no ticket":  {StatusCode: 200, Header: fhttp.Header{"Bd-Ticket-Guard-Server-Data": []string{base64.StdEncoding.EncodeToString([]byte(`{"ts_sign":"x"}`))}}},
			"not json":   {StatusCode: 200, Header: fhttp.Header{"Bd-Ticket-Guard-Server-Data": []string{base64.StdEncoding.EncodeToString([]byte("plain"))}}},
		} {
			if info := ParseTicketGuardServerData(resp); info != nil {
				t.Fatalf("%s: got %#v, want nil", name, info)
			}
		}
	})

	t.Run("ApplyTicketGuard", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{})
		resp := &Response{StatusCode: 200, Header: fhttp.Header{}}
		resp.Header.Set("bd-ticket-guard-server-data", encoded)
		if !ApplyTicketGuard(c, resp) {
			t.Fatal("ApplyTicketGuard = false")
		}
		if c.Ticket != "TICKET-1" || c.TsSign != "ts.1abc" || c.ClientCert != "CERT-1" {
			t.Fatalf("client = ticket %q ts_sign %q cert %q", c.Ticket, c.TsSign, c.ClientCert)
		}
		empty := &Response{StatusCode: 200, Body: []byte("{}")}
		if ApplyTicketGuard(c, empty) {
			t.Fatal("ApplyTicketGuard without a ticket = true")
		}
	})
}

func TestPassportMergeLoginCookiesDeletesOnEmpty(t *testing.T) {
	c, _ := newStubJSON(t, map[string]any{})
	c.Cookie.Set("keep", "old")
	c.Cookie.Set("drop", "old")
	resp := &Response{StatusCode: 200, Cookies: []*fhttp.Cookie{
		ck("keep", "new"),
		ck("drop", ""),
		ck("", "ignored"),
	}}
	got := mergeLoginCookies(c, resp)
	if got["keep"] != "new" || got["drop"] != "" {
		t.Fatalf("merged = %v", got)
	}
	if c.Cookie.Get("keep") != "new" {
		t.Fatalf("keep = %q", c.Cookie.Get("keep"))
	}
	if c.Cookie.Has("drop") {
		t.Fatal("an empty Set-Cookie value must delete the cookie")
	}
}

// ---------------------------------------------------------------------------
// login.go — pure helpers
// ---------------------------------------------------------------------------

func TestLoginFormatSMSPhone(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		bad  bool
	}{
		{"13800138000", "+86 13800138000", false},
		{" 13800138000 ", "+86 13800138000", false},
		{"+8613800138000", "+86 13800138000", false},
		{"+86 13800138000", "+86 13800138000", false},
		{"", "", true},
		{"1380013800", "", true},       // 10 digits
		{"138001380000", "", true},     // 12 digits
		{"1380013800x", "", true},      // non-digit
		{"+861380013800", "", true},    // +86 + 10 digits
		{"+86138001380001", "", true},  // +86 + 12 digits
		{"+86 1380013800", "", true},   // "+86 " + 10 digits
		{"+86138001380000 ", "", true}, // still 14 chars without the space
	} {
		got, err := formatSMSPhone(tc.in)
		if tc.bad {
			if err == nil {
				t.Fatalf("formatSMSPhone(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("formatSMSPhone(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}

	for _, tc := range []struct {
		in  string
		bad bool
	}{{"123456", false}, {" 123456 ", false}, {"12345", true}, {"1234567", true}, {"12345a", true}, {"", true}} {
		if _, err := formatSMSCode(tc.in); (err != nil) != tc.bad {
			t.Fatalf("formatSMSCode(%q) error = %v, bad=%v", tc.in, err, tc.bad)
		}
	}
}

func TestLoginPassportSignFixedPayload(t *testing.T) {
	query := NewParams()
	query.Add("ts", "1700000000")
	query.Add("aid", "6383")
	query.Add("extra2", "b")
	query.Add("msToken", "IGNORED")
	query.Add("sign", "OLD-SIGN")
	query.Add("qs", "OLD-QS")
	query.Add("extra1", "a")
	query.Add("a_bogus", "IGNORED")
	data := NewParams()
	data.Add("token", "XYZ")
	data.Add("need_logo", "false")

	sign, qs := passportSign(query, data)
	const wantSign = "2e3a1b5b0a03432ab0547964d6e971bf294844a56f3043e58a10369f0d457899"
	if sign != wantSign {
		t.Fatalf("sign = %q, want %q", sign, wantSign)
	}
	// qs is the XOR-encoded, lexicographically sorted, first-10 key list;
	// sign/qs/msToken/a_bogus are excluded from the signature.
	if want := PassportEncrypt("aid,extra1,extra2,ts"); qs != want {
		t.Fatalf("qs = %q, want %q", qs, want)
	}

	sign2, qs2 := passportSign(query, data)
	if sign2 != sign || qs2 != qs {
		t.Fatalf("passportSign is not deterministic: %q/%q", sign2, qs2)
	}

	// only the first 10 sorted keys are signed.
	many := NewParams()
	for i := range 12 {
		many.Add("k"+strconv.Itoa(i/10)+strconv.Itoa(i%10), "v")
	}
	_, qsMany := passportSign(many, nil)
	if want := PassportEncrypt("k00,k01,k02,k03,k04,k05,k06,k07,k08,k09"); qsMany != want {
		t.Fatalf("qs (12 keys) = %q, want %q", qsMany, want)
	}
}

func TestLoginAidSignFixedPayload(t *testing.T) {
	got := aidSign("/passport/web/get_qrcode/", "1700000000", "6383")
	const want = "f16c8df37a3c7bb7cfcaa06c934b02927f3488116bffc161a00fd1782e3c30d5"
	if got != want {
		t.Fatalf("aidSign = %q, want %q", got, want)
	}
	if again := aidSign("/passport/web/get_qrcode/", "1700000000", "6383"); again != got {
		t.Fatal("aidSign must be deterministic")
	}
	if other := aidSign("/passport/web/check_qrconnect/", "1700000000", "6383"); other == got {
		t.Fatal("path must be part of the signed message")
	}
	if other := aidSign("/passport/web/get_qrcode/", "1700000000", "1234"); other == got {
		t.Fatal("aid must be part of the signed message")
	}
}

func TestLoginGeneratePassportAuthMixStateAlphabetAndLength(t *testing.T) {
	const alphabet = "1234567890qwertyuiopasdfghjklzxcvbnm"
	for _, n := range []int{0, 1, 8, 32, 64} {
		got := generatePassportAuthMixState(n)
		if len(got) != n {
			t.Fatalf("generatePassportAuthMixState(%d) len = %d", n, len(got))
		}
		for _, r := range got {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("generatePassportAuthMixState(%d) = %q has %q outside the alphabet", n, got, r)
			}
		}
	}
	if a, b := generatePassportAuthMixState(32), generatePassportAuthMixState(32); a == b {
		t.Fatal("mix state must be random per call")
	}
}

func TestLoginKeyMaterialHelpers(t *testing.T) {
	// HMAC-SHA256 RFC-style vector: proves the key material helper is a plain
	// HMAC over the raw key bytes.
	got := hex.EncodeToString(hmacSHABytes([]byte("key"), []byte("The quick brown fox jumps over the lazy dog")))
	const want = "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8"
	if got != want {
		t.Fatalf("hmacSHABytes = %q, want %q", got, want)
	}

	for _, n := range []int{0, 1, 16, 32} {
		b := randomHex(n)
		if len(b) != 2*n {
			t.Fatalf("randomHex(%d) len = %d", n, len(b))
		}
		if _, err := hex.DecodeString(b); err != nil {
			t.Fatalf("randomHex(%d) = %q is not hex: %v", n, b, err)
		}
	}
}

// ---------------------------------------------------------------------------
// login.go — QR request shape through the stub transport
// ---------------------------------------------------------------------------

func TestLoginCreateQRCodeRequestShape(t *testing.T) {
	c, st := newStubClient(t, func(stubRequest) (*Response, error) {
		return jsonResponse(map[string]any{"data": map[string]any{"status": "new"}}), nil
	})
	c.SetMsToken("PINNED-MS")
	c.Cookie.Set("ttwid", "TTWID-1")
	c.Cookie.Set("passport_csrf_token", "CSRF-1")
	cuBootstrapped(c)

	res, err := c.CreateLoginQRCode(t.Context())
	if err != nil {
		t.Fatalf("CreateLoginQRCode: %v", err)
	}
	data, _ := res["data"].(map[string]any)
	if data["status"] != "new" {
		t.Fatalf("decoded data = %v", res)
	}

	reqs := st.requests()
	if len(reqs) != 1 {
		t.Fatalf("bootstrapped session must issue exactly one request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Method != fhttp.MethodGet {
		t.Fatalf("method = %s", req.Method)
	}
	if !strings.HasPrefix(req.URL, passportAPI+"get_qrcode/?") {
		t.Fatalf("url = %s", req.URL)
	}
	qv := cuQuery(t, req.URL)
	for key, want := range map[string]string{
		"aid":                    "6383",
		"language":               "zh",
		"msToken":                "PINNED-MS",
		"next":                   homeBase,
		"need_short_url":         "true",
		"need_logo":              "false",
		"is_new_login":           "1",
		"is_from_iesaccountsaas": "1",
		"passport_jssdk_version": "3.4.4",
		"device_platform":        "web_app",
	} {
		if got := qv.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
	if qv.Get("sign") == "" || qv.Get("qs") == "" || qv.Get("a_bogus") == "" || qv.Get("ts") == "" {
		t.Fatalf("signed query missing material: %s", req.URL)
	}
	// deviceFP is false until a QR refresh is needed.
	if qv.Get("p_ca") != "" || qv.Get("fp") != "" {
		t.Fatalf("fresh session must not send device fp params: %s", req.URL)
	}
	if req.Header("referer") != homeBase+"/" {
		t.Fatalf("referer = %q", req.Header("referer"))
	}
	if req.Header("x-tt-passport-csrf-token") != "CSRF-1" {
		t.Fatalf("csrf header = %q", req.Header("x-tt-passport-csrf-token"))
	}
	if got := req.Header("x-tt-passport-aid-sign"); got == "" {
		t.Fatal("x-tt-passport-aid-sign must be present")
	}
	if got := req.Header("content-type"); got != "" {
		t.Fatalf("GET must not carry a content-type, got %q", got)
	}
	if !strings.Contains(req.Cookie, "ttwid=TTWID-1") || !strings.Contains(req.Cookie, "sessionid=abc") {
		t.Fatalf("cookie header = %q", req.Cookie)
	}
}

func TestLoginCheckQRCodeStatusRequestShapeAndErrorMapping(t *testing.T) {
	t.Run("request shape", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"data": map[string]any{"status": "new"}}), nil
		})
		c.SetMsToken("PINNED-MS")
		c.Cookie.Set("ttwid", "TTWID-1")
		cuBootstrapped(c)

		res, err := c.CheckQRCodeStatus(t.Context(), "TOK123")
		if err != nil {
			t.Fatalf("CheckQRCodeStatus: %v", err)
		}
		if data, _ := res["data"].(map[string]any); data["status"] != "new" {
			t.Fatalf("res = %v", res)
		}
		reqs := st.requests()
		if len(reqs) != 1 {
			t.Fatalf("got %d requests", len(reqs))
		}
		req := reqs[0]
		if req.Method != fhttp.MethodPost || !strings.HasPrefix(req.URL, passportAPI+"check_qrconnect/?") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		qv := cuQuery(t, req.URL)
		if qv.Get("aid") != "6383" || qv.Get("is_from_iesaccountsaas") != "1" {
			t.Fatalf("query = %s", req.URL)
		}
		const wantBody = "need_logo=false&is_frontier=true&token=TOK123&is_new_login=1&next=" +
			"https%3A%2F%2Fwww.douyin.com&need_short_url=true"
		if got := string(req.Body); got != wantBody {
			t.Fatalf("body = %q, want %q", got, wantBody)
		}
		if req.Header("content-type") != "application/x-www-form-urlencoded" {
			t.Fatalf("content-type = %q", req.Header("content-type"))
		}
		// the QR header order (passportHeaderOrderQR) deliberately omits
		// content-length; the transport supplies it.
		if got := req.Header("content-length"); got != "" {
			t.Fatalf("QR requests must not set content-length, got %q", got)
		}
		if req.Header("origin") != homeBase || req.Header("referer") != homeBase+"/" {
			t.Fatalf("origin/referer = %q/%q", req.Header("origin"), req.Header("referer"))
		}
		if !strings.Contains(req.Cookie, "ttwid=TTWID-1") {
			t.Fatalf("cookie header = %q", req.Cookie)
		}
		if data, _ := res["data"].(map[string]any); data == nil {
			t.Fatal("decoded data must be present")
		}
	})

	t.Run("expired arms a refresh", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{"data": map[string]any{"status": "expired", "error_code": 0}})
		c.SetMsToken("PINNED-MS")
		c.Cookie.Set("passport_auth_mix_state", "short")
		cuBootstrapped(c)

		res, err := c.CheckQRCodeStatus(t.Context(), "TOK-EXPIRED")
		if err != nil {
			t.Fatalf("expired status must not error: %v", err)
		}
		if data, _ := res["data"].(map[string]any); data["status"] != "expired" {
			t.Fatalf("res = %v", res)
		}
		if !c.Cookie.Has("download_guide") {
			t.Fatal("expired status must seed download_guide")
		}
		mix := c.Cookie.Get("passport_auth_mix_state")
		if len(mix) != 32 {
			t.Fatalf("mix state = %q (len %d), want a regenerated 32-char value", mix, len(mix))
		}
		// the next QR fetch now carries the fingerprint params.
		_, err = c.CreateLoginQRCode(t.Context())
		if err != nil {
			t.Fatalf("CreateLoginQRCode: %v", err)
		}
		reqs := st.requests()
		last := reqs[len(reqs)-1]
		if !strings.HasPrefix(last.URL, passportAPI+"get_qrcode/?") {
			t.Fatalf("last request = %s", last.URL)
		}
		qv := cuQuery(t, last.URL)
		if qv.Get("p_ca") == "" || qv.Get("fp") == "" {
			t.Fatalf("post-refresh QR fetch must carry fp params: %s", last.URL)
		}
	})

	t.Run("empty body retries", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, ""), nil
		})
		c.SetMsToken("PINNED-MS")
		cuBootstrapped(c)
		res, err := c.CheckQRCodeStatus(t.Context(), "TOK")
		if err != nil {
			t.Fatalf("empty body must be a retry, got %v", err)
		}
		if res["message"] != "retry" {
			t.Fatalf("res = %v", res)
		}
		data, _ := res["data"].(map[string]any)
		if toInt64(data["error_code"]) != 7 {
			t.Fatalf("error_code = %v", data["error_code"])
		}
	})

	t.Run("non JSON is an error", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "<html>login risk page</html>"), nil
		})
		c.SetMsToken("PINNED-MS")
		cuBootstrapped(c)
		_, err := c.CheckQRCodeStatus(t.Context(), "TOK")
		if err == nil || !strings.Contains(err.Error(), "非 JSON") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("risk error code", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"data": map[string]any{"error_code": 4031, "description": "risk"}})
		c.SetMsToken("PINNED-MS")
		cuBootstrapped(c)
		res, err := c.CheckQRCodeStatus(t.Context(), "TOK")
		if err == nil || !strings.Contains(err.Error(), "4031") {
			t.Fatalf("error = %v (res %v)", err, res)
		}
	})
}

func TestLoginCookieStrRoundTripAfterScriptedResponse(t *testing.T) {
	c, st := newStubClient(t, func(stubRequest) (*Response, error) {
		return &Response{
			StatusCode: 200,
			Body:       []byte(`{"data":{"status":"new"}}`),
			Cookies:    []*fhttp.Cookie{ck("ttwid", "T9"), ck("sessionid_ss", "SS9"), ck("biz_trace_id", "")},
		}, nil
	})
	c.SetMsToken("PINNED-MS")
	cuBootstrapped(c)

	if _, err := c.CreateLoginQRCode(t.Context()); err != nil {
		t.Fatalf("CreateLoginQRCode: %v", err)
	}
	if got := c.Cookie.Get("ttwid"); got != "T9" {
		t.Fatalf("ttwid = %q", got)
	}
	if got := c.Cookie.Get("sessionid_ss"); got != "SS9" {
		t.Fatalf("sessionid_ss = %q", got)
	}
	if c.Cookie.Has("biz_trace_id") {
		t.Fatal("empty Set-Cookie must delete biz_trace_id")
	}

	serialized := c.CookieStr()
	if !strings.Contains(serialized, "ttwid=T9") || !strings.Contains(serialized, "sessionid_ss=SS9") {
		t.Fatalf("CookieStr = %q", serialized)
	}
	if strings.Contains(serialized, "biz_trace_id") {
		t.Fatalf("CookieStr must not resurrect deleted cookies: %q", serialized)
	}

	// round trip: rebuilding a session from the serialized string preserves
	// the login cookies and stays usable against the same transport.
	round, err := NewClient(serialized, Options{Transport: st})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if round.Cookie.Get("ttwid") != "T9" || round.Cookie.Get("sessionid_ss") != "SS9" {
		t.Fatalf("round-tripped cookies = %q", round.CookieStr())
	}
	if !strings.Contains(round.CookieStr(), "sessionid=abc") {
		t.Fatalf("round-trip lost the original session cookie: %q", round.CookieStr())
	}
}

// ---------------------------------------------------------------------------
// node.go / acrawler.go — Node resolution fallback
// ---------------------------------------------------------------------------

func TestNodeResolutionExplicitEnvBeatsPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable shims only")
	}
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit-node")
	cuWriteExec(t, explicit)
	pathDir := t.TempDir()
	cuWriteExec(t, filepath.Join(pathDir, "node"))

	t.Setenv("DOUYIN_NODE", "")
	t.Setenv("PATH", pathDir)
	t.Setenv("DY_NODE", explicit)
	if got := resolveNode(); got != explicit {
		t.Fatalf("resolveNode = %q, want the DY_NODE override %q", got, explicit)
	}
	if !NodeAvailable() {
		t.Fatal("NodeAvailable must be true when DY_NODE points at an executable")
	}

	// clearing the override falls back to the node on PATH.
	t.Setenv("DY_NODE", "")
	if got := resolveNode(); got != filepath.Join(pathDir, "node") {
		t.Fatalf("resolveNode = %q, want the PATH node", got)
	}
	if _, err := requireNode("page acrawler execution"); err != nil {
		t.Fatalf("requireNode = %v with a node available", err)
	}
}

func TestNodeResolutionFallbackWithoutNode(t *testing.T) {
	cuNoNode(t)
	if got := resolveNode(); got != "" {
		t.Fatalf("resolveNode = %q, want empty when DY_NODE is empty and PATH has no node/mise", got)
	}
	if NodeAvailable() {
		t.Fatal("NodeAvailable = true without a node binary")
	}
	path, err := requireNode("page acrawler execution")
	if err == nil {
		t.Fatalf("requireNode must fail, got path %q", path)
	}
	for _, want := range []string{"page acrawler execution", "DY_NODE", "mise"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q must mention %q", err, want)
		}
	}
}

func TestAcrawlerRunnerFallsBackWithoutNode(t *testing.T) {
	cuNoNode(t)
	ctx := t.Context()

	// non-strict: a soft synthetic result that keeps the original cookie.
	res, err := GenerateACSignature(ctx, ACOptions{Nonce: "NONCE-1", Cookie: "a=1; b=2"})
	if err != nil {
		t.Fatalf("soft mode must not error without node: %v", err)
	}
	if res["sig"] != "" {
		t.Fatalf("sig = %v, want empty", res["sig"])
	}
	if res["provenance"] != "unproven_synthetic" {
		t.Fatalf("provenance = %v", res["provenance"])
	}
	if res["cookie_header"] != "a=1; b=2" {
		t.Fatalf("cookie_header = %v", res["cookie_header"])
	}
	cookies, _ := res["cookie"].(map[string]any)
	if cookies["a"] != "1" || cookies["b"] != "2" {
		t.Fatalf("cookie map = %v", cookies)
	}

	// strict: the missing-Node error surfaces instead of a fake signature.
	if _, err := GenerateACSignature(ctx, ACOptions{Nonce: "NONCE-1", Strict: true}); err == nil ||
		!strings.Contains(err.Error(), "Node.js is required") {
		t.Fatalf("strict error = %v", err)
	}

	// empty nonce fails before Node resolution in both modes.
	if _, err := GenerateACSignature(ctx, ACOptions{Nonce: "  "}); err != nil {
		t.Fatalf("empty nonce soft mode = %v", err)
	}
	if _, err := GenerateACSignature(ctx, ACOptions{Nonce: "", Strict: true}); err == nil ||
		!strings.Contains(err.Error(), "nonce") {
		t.Fatalf("empty nonce strict error = %v", err)
	}
}

func TestAcrawlerLastJSONLineAndCookieParsing(t *testing.T) {
	got := lastJSONLine("noise\n{\"sig\":\"_abc\"}\ntrailing garbage\n")
	if got["sig"] != "_abc" {
		t.Fatalf("lastJSONLine = %v", got)
	}
	if got := lastJSONLine(""); len(got) != 0 {
		t.Fatalf("empty stdout = %v", got)
	}
	if got := lastJSONLine("{not json}"); len(got) != 0 {
		t.Fatalf("invalid json = %v", got)
	}

	parsed := parseCookieHeader(" a=1 ; b = 2 ; broken ; c= ; =x ")
	if parsed["a"] != "1" || parsed["b"] != "2" || parsed["c"] != "" {
		t.Fatalf("parseCookieHeader = %v", parsed)
	}
	if _, ok := parsed["broken"]; ok {
		t.Fatalf("a part without '=' must be skipped: %v", parsed)
	}
	if len(parsed) != 4 {
		t.Fatalf("parseCookieHeader = %v", parsed)
	}

	if orDefault("", "d") != "d" || orDefault("   ", "d") != "d" || orDefault("v", "d") != "v" {
		t.Fatalf("orDefault = %q/%q/%q", orDefault("", "d"), orDefault("   ", "d"), orDefault("v", "d"))
	}
}
