package douyin

// Hermetic (offline) unit tests for the PC IM rich-media upload path
// (im_media.go), the 私信 WebSocket receiver (im_ws.go), the generic protobuf
// wire helpers (improto.go) and the IM misc panel endpoints (api_misc_im.go).
//
// Every HTTP interaction goes through the shared stub transport and every file
// path lives in t.TempDir(); nothing here touches the network, a browser or
// the clock (beyond the request signatures themselves).

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// imMediaTestPNG encodes a valid width×height PNG so imImageSize can decode it.
func imMediaTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// imMediaTestQuery parses the query string of a recorded request URL.
func imMediaTestQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	return u.Query()
}

// imMediaTestPushFrame hand-assembles a PushFrame (payloadType=7, payload=8).
func imMediaTestPushFrame(payloadType string, payload []byte) []byte {
	w := &pbw{}
	w.Str(7, payloadType)
	w.Bytes(8, payload)
	return w.b
}

// imMediaTestNotifyFrame hand-assembles the bytes a frontier-im push delivers:
// PushFrame{payloadType:"pb", payload:Response{cmd:500, body:ResponseBody{
// new_message_notify: NewMessageNotify{conversation_id, conversation_type,
// notify_type, message: MessageBody{...}}}}}.
func imMediaTestNotifyFrame(notifyConvType, msgConvType int64) []byte {
	msg := &pbw{}
	msg.Str(1, "0:1:111:222")
	msg.IntAlways(2, msgConvType)
	msg.IntAlways(4, 77)  // index_in_conversation
	msg.IntAlways(6, 7)   // message_type
	msg.IntAlways(7, 111) // sender
	msg.Str(8, `{"text":"hi"}`)

	notify := &pbw{}
	notify.Str(2, "0:1:111:222")
	notify.IntAlways(3, notifyConvType)
	notify.IntAlways(4, 50001) // notify_type
	notify.Msg(5, msg)

	body := &pbw{}
	body.Msg(500, notify) // ResponseBody.new_message_notify

	resp := &pbw{}
	resp.IntAlways(1, 500) // cmd
	resp.Msg(6, body)      // Response.body

	return imMediaTestPushFrame("pb", resp.b)
}

// imMediaTestNewMediaClient returns a stub-backed client with deterministic
// webid/msToken so no background bootstrap request is issued.
func imMediaTestNewMediaClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	c, st := newStubClient(t, handle)
	c.SetWebID("webid-test")
	c.SetMsToken("ms-token-test")
	return c, st
}

// ---------------------------------------------------------------------------
// im_media.go
// ---------------------------------------------------------------------------

func TestImMediaSourceReadAndLimits(t *testing.T) {
	ctx := t.Context()

	t.Run("file source", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "clip.bin")
		if err := os.WriteFile(path, []byte("abcdefghij"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			t.Fatal("file source must not issue HTTP")
			return nil, nil
		})
		src, err := imNewMediaSource(ctx, c, path, "unused.bin")
		if err != nil {
			t.Fatalf("imNewMediaSource: %v", err)
		}
		if src.path != path || src.size != 10 || src.name != "clip.bin" {
			t.Fatalf("source = %+v, want path=%q size=10 name=clip.bin", src, path)
		}
		if got, _ := src.read(2, 4); string(got) != "cdef" {
			t.Errorf("read(2,4) = %q, want cdef", got)
		}
		if got, _ := src.read(8, 10); string(got) != "ij" {
			t.Errorf("read(8,10) = %q, want the remainder ij", got)
		}
		if got, err := src.read(10, 1); err != nil || len(got) != 0 {
			t.Errorf("read past EOF = (%q, %v), want (empty, nil)", got, err)
		}
		if len(st.requests()) != 0 {
			t.Errorf("file source issued %d requests", len(st.requests()))
		}
	})

	t.Run("in-memory source", func(t *testing.T) {
		src := &imMediaSource{data: []byte("xyz"), size: 3}
		if got, _ := src.read(0, 2); string(got) != "xy" {
			t.Errorf("read(0,2) = %q, want xy", got)
		}
		if got, _ := src.read(3, 1); got != nil {
			t.Errorf("read past data = %q, want nil", got)
		}
	})

	t.Run("http source", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "hello-media"), nil
		})
		src, err := imNewMediaSource(ctx, c, "https://example.com/media.png", "media.png")
		if err != nil {
			t.Fatalf("imNewMediaSource: %v", err)
		}
		if string(src.data) != "hello-media" || src.size != 11 || src.name != "media.png" {
			t.Fatalf("source = %+v", src)
		}
		req := st.last(t)
		if req.Method != "GET" || req.URL != "https://example.com/media.png" {
			t.Errorf("request = %s %s", req.Method, req.URL)
		}
	})

	t.Run("http source error status", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(404, "gone"), nil
		})
		_, err := imNewMediaSource(ctx, c, "https://example.com/missing.png", "x")
		if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
			t.Fatalf("err = %v, want HTTP 404", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		c, _ := newStubClient(t, nil)
		if _, err := imNewMediaSource(ctx, c, filepath.Join(t.TempDir(), "nope"), "x"); err == nil {
			t.Fatal("want stat error for a missing file")
		}
	})
}

func TestImMediaSliceAndImageHelpers(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want int64
	}{
		{0, 3 * imMB},
		{3 * imMB, 3 * imMB},
		{100*imMB - 1, 3 * imMB},
		{100 * imMB, 5 * imMB},
		{500*imMB - 1, 5 * imMB},
		{500 * imMB, 10 * imMB},
		{5 * 1024 * imMB, 10 * imMB},
	} {
		if got := imSliceSizeFor(tc.size); got != tc.want {
			t.Errorf("imSliceSizeFor(%d) = %d, want %d", tc.size, got, tc.want)
		}
	}

	if got := imCRC32Hex(nil); got != "00000000" {
		t.Errorf("imCRC32Hex(nil) = %s, want 00000000", got)
	}
	// Standard CRC-32/IEEE check value for "123456789".
	if got := imCRC32Hex([]byte("123456789")); got != "cbf43926" {
		t.Errorf("imCRC32Hex(123456789) = %s, want cbf43926", got)
	}

	png3x2 := imMediaTestPNG(t, 3, 2)
	if w, h := imImageSize(png3x2); w != 3 || h != 2 {
		t.Errorf("imImageSize(png) = (%d,%d), want (3,2)", w, h)
	}
	if w, h := imImageSize([]byte("not an image")); w != 0 || h != 0 {
		t.Errorf("imImageSize(garbage) = (%d,%d), want (0,0)", w, h)
	}
}

func TestImMediaVODSigningAndEncoding(t *testing.T) {
	// Fixed key material + RFC vectors: byte-level assertions on the two
	// primitives the ImageX/VOD gateway signature is built from.
	emptyDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := imSHA256Hex(nil); got != emptyDigest {
		t.Errorf("imSHA256Hex(nil) = %s, want %s", got, emptyDigest)
	}
	if got := imSHA256Hex([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("imSHA256Hex(abc) = %s", got)
	}
	// RFC 4231 test case 1.
	key := bytes.Repeat([]byte{0x0b}, 20)
	if got := hex.EncodeToString(imHMAC(key, []byte("Hi There"))); got != "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7" {
		t.Errorf("imHMAC RFC4231 = %s", got)
	}

	// imCanonicalQuery sorts by encoded key then value (AWS SigV4 ordering).
	if got := imCanonicalQuery([]imKV{{"b", "2"}, {"a", "1"}, {"a", "0"}}); got != "a=0&a=1&b=2" {
		t.Errorf("imCanonicalQuery = %q, want a=0&a=1&b=2", got)
	}
	if got := imCanonicalQuery([]imKV{{"k/1", "v 2"}}); got != "k%2F1=v%202" {
		t.Errorf("imCanonicalQuery(special) = %q, want k%%2F1=v%%202", got)
	}
	if got := imCanonicalQuery(nil); got != "" {
		t.Errorf("imCanonicalQuery(nil) = %q, want empty", got)
	}

	// imEncodeKV uses QueryEscape, unlike the strict SigV4 canonical form.
	if got := imEncodeKV([]imKV{{"a b", "c/d"}}); got != "a+b=c%2Fd" {
		t.Errorf("imEncodeKV = %q, want a+b=c%%2Fd", got)
	}

	query := []imKV{{"Action", "CommitUploadInner"}, {"Version", imVODAPIVersion}}
	amzDate := regexp.MustCompile(`^\d{8}T\d{6}Z$`)

	get := imSignVOD("AK", "SK", "TOKEN", "GET", query, nil, imVODService, imRegion)
	if !amzDate.MatchString(get["x-amz-date"]) {
		t.Errorf("GET x-amz-date = %q", get["x-amz-date"])
	}
	if get["x-amz-security-token"] != "TOKEN" {
		t.Errorf("GET token = %q", get["x-amz-security-token"])
	}
	if _, ok := get["x-amz-content-sha256"]; ok {
		t.Error("GET must not sign x-amz-content-sha256")
	}
	if !strings.HasPrefix(get["authorization"], "AWS4-HMAC-SHA256 Credential=AK/") {
		t.Errorf("GET authorization = %q", get["authorization"])
	}
	if !strings.Contains(get["authorization"], "SignedHeaders=x-amz-date;x-amz-security-token") {
		t.Errorf("GET signed headers = %q", get["authorization"])
	}
	if !strings.Contains(get["authorization"], "/cn-north-1/vod/aws4_request") {
		t.Errorf("GET credential scope = %q", get["authorization"])
	}
	if !regexp.MustCompile(`Signature=[0-9a-f]{64}$`).MatchString(get["authorization"]) {
		t.Errorf("GET signature = %q", get["authorization"])
	}

	body := []byte(`{"SessionKey":"sk"}`)
	post := imSignVOD("AK", "SK", "TOKEN", "POST", query, body, imVODService, imRegion)
	if post["x-amz-content-sha256"] != imSHA256Hex(body) {
		t.Errorf("POST payload hash = %q, want %q", post["x-amz-content-sha256"], imSHA256Hex(body))
	}
	if !strings.Contains(post["authorization"], "SignedHeaders=x-amz-content-sha256;x-amz-date;x-amz-security-token") {
		t.Errorf("POST signed headers = %q", post["authorization"])
	}
	if !amzDate.MatchString(post["x-amz-date"]) {
		t.Fatalf("POST x-amz-date = %q, want the RFC1123 basic form", post["x-amz-date"])
	}

	t.Run("gateway headers", func(t *testing.T) {
		signed := map[string]string{
			"authorization":        "AWS4-HMAC-SHA256 ...",
			"x-amz-date":           "20260101T000000Z",
			"x-amz-security-token": "TOKEN",
			"x-amz-content-sha256": "hash",
			"x-not-allowed":        "nope",
		}
		h := imGatewayHeaders(signed, "text/plain;charset=UTF-8")
		for k, want := range signed {
			if k == "x-not-allowed" {
				continue
			}
			if got, _ := h.Get(k); got != want {
				t.Errorf("header %s = %q, want %q", k, got, want)
			}
		}
		if v, _ := h.Get("x-not-allowed"); v != "" {
			t.Errorf("unsigned header leaked: %q", v)
		}
		if v, _ := h.Get("content-type"); v != "text/plain;charset=UTF-8" {
			t.Errorf("content-type = %q", v)
		}
		if v, _ := h.Get("origin"); v != imOrigin {
			t.Errorf("origin = %q", v)
		}
		if v, _ := h.Get("referer"); v != imOrigin+"/" {
			t.Errorf("referer = %q", v)
		}
		if _, ok := imGatewayHeaders(map[string]string{}, "").Get("content-type"); ok {
			t.Error("empty contentType must not set content-type")
		}
	})

	t.Run("tos headers", func(t *testing.T) {
		node := imUploadNode{Auth: "tos-auth", UploadHeader: map[string]string{"x-extra": "7"}}
		h := imTOSHeaders(node, "u 1", "crc-1")
		if v, _ := h.Get("authorization"); v != "tos-auth" {
			t.Errorf("authorization = %q", v)
		}
		if v, _ := h.Get("x-storage-u"); v != "u+1" {
			t.Errorf("x-storage-u = %q, want u+1", v)
		}
		if v, _ := h.Get("content-crc32"); v != "crc-1" {
			t.Errorf("content-crc32 = %q", v)
		}
		if v, _ := h.Get("x-extra"); v != "7" {
			t.Errorf("node upload header not propagated: %q", v)
		}
		if _, ok := imTOSHeaders(node, "u", "").Get("content-crc32"); ok {
			t.Error("empty crc must not set content-crc32")
		}
	})
}

func TestImMediaApplyUploadRequestShape(t *testing.T) {
	ctx := t.Context()
	sts := imSTS{AccessKeyID: "AK", SecretAccessKey: "SK", SessionToken: "TOK", SpaceName: "space-1"}
	payload := map[string]any{
		"Result": map[string]any{
			"InnerUploadAddress": map[string]any{
				"UploadNodes": []any{map[string]any{
					"StoreInfos":   []any{map[string]any{"StoreUri": "store/uri", "Auth": "node-auth", "UploadID": "up-1"}},
					"UploadHost":   "tos.example.com",
					"SessionKey":   "sk-1",
					"UploadHeader": map[string]any{"x-extra": "v"},
				}},
			},
		},
	}

	c, st := newStubJSON(t, payload)
	node, err := c.imApplyUpload(ctx, sts, "video", 12345, "u-1", true)
	if err != nil {
		t.Fatalf("imApplyUpload: %v", err)
	}
	req := st.last(t)
	if req.Method != "GET" {
		t.Errorf("method = %s, want GET", req.Method)
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	if u.Host != imVODHost || u.Path != "/" {
		t.Errorf("url = %s, want host %s path /", req.URL, imVODHost)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"Action":       "ApplyUploadInner",
		"Version":      imVODAPIVersion,
		"SpaceName":    "space-1",
		"FileType":     "video",
		"IsInner":      "1",
		"NeedFallback": "true",
		"FileSize":     "12345",
		"OpenGcmEnc":   "true",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}
	if req.Header("x-amz-security-token") != "TOK" || req.Header("authorization") == "" {
		t.Errorf("gateway signing headers missing: %v", st.last(t).Headers)
	}
	if ct := req.Header("content-type"); ct != "" {
		t.Errorf("content-type = %q, want unset for a GET apply", ct)
	}
	if node.StoreURI != "store/uri" || node.Auth != "node-auth" || node.UploadID != "up-1" ||
		node.UploadHost != "tos.example.com" || node.SessionKey != "sk-1" || node.UploadHeader["x-extra"] != "v" {
		t.Errorf("node = %+v", node)
	}

	// gcm=false must drop OpenGcmEnc (file/image uploads do not request it).
	c2, st2 := newStubJSON(t, payload)
	if _, err := c2.imApplyUpload(ctx, sts, "image", 1, "u-1", false); err != nil {
		t.Fatalf("imApplyUpload(image): %v", err)
	}
	q2 := imMediaTestQuery(t, st2.last(t).URL)
	if got := q2.Get("FileType"); got != "image" {
		t.Errorf("FileType = %q, want image", got)
	}
	if _, ok := q2["OpenGcmEnc"]; ok {
		t.Error("OpenGcmEnc must be absent when gcm=false")
	}

	t.Run("no upload nodes", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"Result": map[string]any{}})
		_, err := c.imApplyUpload(ctx, sts, "image", 1, "u", false)
		if err == nil || !strings.Contains(err.Error(), "ApplyUploadInner(image) 失败") {
			t.Fatalf("err = %v, want ApplyUploadInner(image) 失败", err)
		}
	})

	t.Run("no store infos", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"Result": map[string]any{
			"InnerUploadAddress": map[string]any{"UploadNodes": []any{map[string]any{"UploadHost": "h"}}},
		}})
		_, err := c.imApplyUpload(ctx, sts, "object", 1, "u", false)
		if err == nil || !strings.Contains(err.Error(), "缺少 StoreInfos") {
			t.Fatalf("err = %v, want 缺少 StoreInfos", err)
		}
	})

	t.Run("non json", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "<html>"), nil
		})
		_, err := c.imApplyUpload(ctx, sts, "image", 1, "u", false)
		if err == nil || !strings.Contains(err.Error(), "非 JSON") {
			t.Fatalf("err = %v, want non-JSON error", err)
		}
	})
}

func TestImMediaUploadSourceChunking(t *testing.T) {
	ctx := t.Context()

	t.Run("direct upload", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{"code": "2000"})
		node := imUploadNode{UploadHost: "tos.example.com", StoreURI: "store/uri", Auth: "node-auth"}
		data := []byte("hello-media")
		src := &imMediaSource{data: data, size: int64(len(data))}
		if err := c.imUploadSource(ctx, node, src, "u 1"); err != nil {
			t.Fatalf("imUploadSource: %v", err)
		}
		if n := len(st.requests()); n != 1 {
			t.Fatalf("requests = %d, want 1 (direct upload)", n)
		}
		req := st.last(t)
		if req.Method != "POST" || req.URL != "https://tos.example.com/upload/v1/store/uri" {
			t.Errorf("request = %s %s", req.Method, req.URL)
		}
		if !bytes.Equal(req.Body, data) {
			t.Errorf("body = %q, want %q", req.Body, data)
		}
		if got := req.Header("content-crc32"); got != imCRC32Hex(data) {
			t.Errorf("content-crc32 = %q, want %q", got, imCRC32Hex(data))
		}
		if got := req.Header("x-storage-u"); got != "u+1" {
			t.Errorf("x-storage-u = %q", got)
		}
		if got := req.Header("authorization"); got != "node-auth" {
			t.Errorf("authorization = %q", got)
		}
	})

	t.Run("init then finish", func(t *testing.T) {
		c, st := newStubClient(t, func(req stubRequest) (*Response, error) {
			if strings.Contains(req.URL, "phase=init") {
				return jsonResponse(map[string]any{"data": map[string]any{"uploadid": "uid-9"}}), nil
			}
			return jsonResponse(map[string]any{"code": "2000"}), nil
		})
		node := imUploadNode{UploadHost: "tos.example.com", StoreURI: "store/uri", Auth: "a"}
		data := make([]byte, imDirectUploadMax+1) // 3MB+1 → one 5MB part
		src := &imMediaSource{data: data, size: int64(len(data))}
		if err := c.imUploadSource(ctx, node, src, "u"); err != nil {
			t.Fatalf("imUploadSource: %v", err)
		}
		reqs := st.requests()
		if len(reqs) != 3 {
			t.Fatalf("requests = %d, want init+transfer+finish", len(reqs))
		}
		if !strings.Contains(reqs[0].URL, "?uploadmode=part&phase=init") {
			t.Errorf("init URL = %s", reqs[0].URL)
		}
		if !strings.Contains(reqs[1].URL, "uploadid=uid-9") || !strings.Contains(reqs[1].URL, "part_number=1") ||
			!strings.Contains(reqs[1].URL, "phase=transfer") || !strings.Contains(reqs[1].URL, "part_offset=0") {
			t.Errorf("transfer URL = %s", reqs[1].URL)
		}
		if len(reqs[1].Body) != len(data) {
			t.Errorf("transfer body = %d bytes, want %d", len(reqs[1].Body), len(data))
		}
		if !strings.Contains(reqs[2].URL, "phase=finish&uploadid=uid-9") {
			t.Errorf("finish URL = %s", reqs[2].URL)
		}
		if want := "1:" + imCRC32Hex(data); string(reqs[2].Body) != want {
			t.Errorf("finish body = %q, want %q", reqs[2].Body, want)
		}
	})

	t.Run("multipart merge", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{"code": "2000"})
		node := imUploadNode{UploadHost: "tos.example.com", StoreURI: "store/uri", Auth: "a", UploadID: "uid-m"}
		data := make([]byte, 5*imMB+1) // 5MB floor → two parts (5MB + 1 byte)
		src := &imMediaSource{data: data, size: int64(len(data))}
		if err := c.imUploadSource(ctx, node, src, "u"); err != nil {
			t.Fatalf("imUploadSource: %v", err)
		}
		reqs := st.requests()
		if len(reqs) != 3 {
			t.Fatalf("requests = %d, want two transfers + finish", len(reqs))
		}
		for i, req := range reqs[:2] {
			wantPart := fmt.Sprintf("part_number=%d", i+1)
			wantOff := fmt.Sprintf("part_offset=%d", int64(i)*5*imMB)
			if !strings.Contains(req.URL, wantPart) || !strings.Contains(req.URL, wantOff) {
				t.Errorf("transfer %d URL = %s, want %s %s", i, req.URL, wantPart, wantOff)
			}
		}
		want := fmt.Sprintf("1:%s,2:%s", imCRC32Hex(data[:5*imMB]), imCRC32Hex(data[5*imMB:]))
		if string(reqs[2].Body) != want {
			t.Errorf("finish body = %q, want %q", reqs[2].Body, want)
		}
	})

	t.Run("init without uploadid", func(t *testing.T) {
		c, _ := newStubClient(t, func(req stubRequest) (*Response, error) {
			return jsonResponse(map[string]any{"data": map[string]any{}}), nil
		})
		node := imUploadNode{UploadHost: "h", StoreURI: "s"}
		data := make([]byte, imDirectUploadMax+1)
		err := c.imUploadSource(ctx, node, &imMediaSource{data: data, size: int64(len(data))}, "u")
		if err == nil || !strings.Contains(err.Error(), "IM 分片初始化失败") {
			t.Fatalf("err = %v, want 分片初始化失败", err)
		}
	})

	t.Run("tos rejection", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"code": "5001", "message": "denied"})
		node := imUploadNode{UploadHost: "h", StoreURI: "s"}
		err := c.imUploadSource(ctx, node, &imMediaSource{data: []byte("x"), size: 1}, "u")
		if err == nil || !strings.Contains(err.Error(), "IM TOS 上传失败") {
			t.Fatalf("err = %v, want TOS failure", err)
		}
	})
}

func TestImMediaRejectsOversizeAndMissingThumb(t *testing.T) {
	ctx := t.Context()

	t.Run("file over 10MB", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{})
		path := filepath.Join(t.TempDir(), "big.bin")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.Truncate(path, imMaxFileSize+1); err != nil {
			t.Fatalf("Truncate: %v", err)
		}
		_, err := c.imUploadFile(ctx, path)
		if err == nil || !strings.Contains(err.Error(), "10MB") {
			t.Fatalf("err = %v, want the 10MB rejection", err)
		}
		for _, req := range st.requests() {
			if strings.Contains(req.URL, imUploadConfigPath) {
				t.Fatalf("oversize file still fetched the upload config: %s", req.URL)
			}
		}
	})

	t.Run("video without thumb", func(t *testing.T) {
		c, st := newStubJSON(t, map[string]any{})
		path := filepath.Join(t.TempDir(), "clip.mp4")
		if err := os.WriteFile(path, []byte("v"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := c.imUploadVideo(ctx, path, "")
		if err == nil || !strings.Contains(err.Error(), "thumb") {
			t.Fatalf("err = %v, want the thumb requirement", err)
		}
		for _, req := range st.requests() {
			if strings.Contains(req.URL, imUploadConfigPath) {
				t.Fatalf("thumbless video still fetched the upload config: %s", req.URL)
			}
		}
	})

	t.Run("resolved user id", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "<html>risk</html>"), nil
		})
		if got := c.imResolveUserID(ctx); got != "" {
			t.Errorf("unresolvable uid = %q, want empty", got)
		}
		c.SetUID(1234567)
		if got := c.imResolveUserID(ctx); got != "1234567" {
			t.Errorf("pinned uid = %q, want 1234567", got)
		}
	})
}

func TestImMediaHelpers(t *testing.T) {
	t.Run("truthy and int coercion", func(t *testing.T) {
		for _, tc := range []struct {
			in   any
			want bool
		}{
			{nil, false},
			{true, true},
			{false, false},
			{"", false},
			{"0", false},
			{"x", true},
			{float64(0), false},
			{float64(0.5), true},
			{json.Number("0"), false},
			{json.Number("2"), true},
			{struct{}{}, true},
		} {
			if got := imTruthy(tc.in); got != tc.want {
				t.Errorf("imTruthy(%v) = %v, want %v", tc.in, got, tc.want)
			}
		}
		for _, tc := range []struct {
			in   any
			want int64
		}{
			{nil, 9},
			{json.Number("42"), 42},
			{json.Number("bad"), 9},
			{int64(7), 7},
			{int(8), 8},
			{float64(3.9), 3},
			{"12", 12},
			{" 13 ", 13},
			{"42.9", 42},
			{"abc", 9},
		} {
			if got := imAsIntDefault(tc.in, 9); got != tc.want {
				t.Errorf("imAsIntDefault(%v) = %d, want %d", tc.in, got, tc.want)
			}
		}
	})

	t.Run("sts normalization", func(t *testing.T) {
		cfg := map[string]any{
			"access_key_id":     "A",
			"secret_access_key": "S",
			"session_token":     "T",
			"space_name":        "P",
			"expire_at":         json.Number("99"),
		}
		sts, err := imNormalizeSTS(cfg)
		if err != nil {
			t.Fatalf("imNormalizeSTS: %v", err)
		}
		if sts.AccessKeyID != "A" || sts.SecretAccessKey != "S" || sts.SessionToken != "T" ||
			sts.SpaceName != "P" || sts.ExpireAt != 99 {
			t.Fatalf("sts = %+v", sts)
		}
		// ExpiredTime is the legacy fallback for expire_at.
		if got, _ := imNormalizeSTS(map[string]any{
			"AccessKeyID": "A", "SecretAccessKey": "S", "SessionToken": "T", "SpaceName": "P",
			"ExpiredTime": "123",
		}); got.ExpireAt != 123 {
			t.Errorf("ExpiredTime fallback = %d, want 123", got.ExpireAt)
		}
		if _, err := imNormalizeSTS(map[string]any{"access_key_id": "A"}); err == nil ||
			!strings.Contains(err.Error(), "不完整") {
			t.Fatalf("err = %v, want 字段不完整", err)
		}
		if _, err := imNormalizeSTS(map[string]any{}); err == nil ||
			!strings.Contains(err.Error(), "[]") {
			t.Fatalf("empty cfg err = %v, want sorted key list", err)
		}
		if got := imSTSValue(map[string]any{"a": "", "b": "x"}, "a", "b"); got != "x" {
			t.Errorf("imSTSValue = %q, want x", got)
		}
		if got := imMapKeys(map[string]any{"b": 1, "a": 2}); strings.Join(got, ",") != "a,b" {
			t.Errorf("imMapKeys = %v, want [a b]", got)
		}
		config := map[string]any{"public_image_config": map[string]any{
			"AccessKeyID": "A", "SecretAccessKey": "S", "SessionToken": "T", "space_name": "P",
		}}
		if got, err := imSTSFromConfig(config, "public_image_config"); err != nil || got.SpaceName != "P" {
			t.Errorf("imSTSFromConfig = (%+v, %v)", got, err)
		}
		if _, err := imSTSFromConfig(config, "missing"); err == nil {
			t.Error("missing config entry must error")
		}
	})

	t.Run("commit result extraction", func(t *testing.T) {
		item := map[string]any{"Uri": "u"}
		if got := imResultItem(map[string]any{"Result": map[string]any{"Results": []any{item}}}); got["Uri"] != "u" {
			t.Errorf("imResultItem(Results) = %v", got)
		}
		if got := imResultItem(map[string]any{"Result": map[string]any{"Uri": "direct"}}); got["Uri"] != "direct" {
			t.Errorf("imResultItem(Result) = %v", got)
		}
		if got := imResultItem(map[string]any{}); got != nil {
			t.Errorf("imResultItem(empty) = %v, want nil", got)
		}
	})

	t.Run("plain uri precedence", func(t *testing.T) {
		if got := imPlainURI(map[string]any{
			"Encryption": map[string]any{"Uri": "enc"},
			"Uri":        "plain",
		}); got != "enc" {
			t.Errorf("imPlainURI = %q, want enc", got)
		}
		if got := imPlainURI(map[string]any{"Uri": "plain"}); got != "plain" {
			t.Errorf("imPlainURI = %q, want plain", got)
		}
		if got := imPlainURI(map[string]any{"uri": "lower"}); got != "lower" {
			t.Errorf("imPlainURI = %q, want lower", got)
		}
		if got := imPlainURI(map[string]any{}); got != "" {
			t.Errorf("imPlainURI(empty) = %q", got)
		}
		if enc := imEncryption(map[string]any{}); enc != nil {
			t.Errorf("imEncryption(empty) = %v, want nil", enc)
		}
	})

	t.Run("image content", func(t *testing.T) {
		data := imMediaTestPNG(t, 3, 2)
		item := map[string]any{"Encryption": map[string]any{
			"Uri": "oid-1", "SourceMd5": "md5-1", "SecretKey": "skey-1",
			"Extra": map[string]any{"img_width": 100, "img_height": 50, "img_size": 9999},
		}}
		got := imImageContent(item, data, false)
		res, _ := got["resource_url"].(map[string]any)
		if res["oid"] != "oid-1" || res["skey"] != "skey-1" || res["md5"] != "md5-1" ||
			imAsIntDefault(res["data_size"], -1) != 9999 {
			t.Errorf("resource_url = %v", res)
		}
		if imAsIntDefault(got["cover_width"], -1) != 100 || imAsIntDefault(got["cover_height"], -1) != 50 {
			t.Errorf("cover = %v x %v, want 100x50", got["cover_width"], got["cover_height"])
		}
		if imAsIntDefault(got["aweType"], -1) != 2702 || got["md5"] != "md5-1" ||
			imAsIntDefault(got["from_gallery"], -1) != 1 {
			t.Errorf("payload = %v", got)
		}
		if list, ok := got["check_pics"].([]any); !ok || len(list) != 0 {
			t.Errorf("check_pics = %v", got["check_pics"])
		}
		if gif := imImageContent(item, data, true); imAsIntDefault(gif["aweType"], -1) != 2703 {
			t.Errorf("gif aweType = %v, want 2703", gif["aweType"])
		}
		// Without Encryption the image's own dimensions and md5 are used.
		plain := imImageContent(map[string]any{"Uri": "u", "SourceMd5": "m", "SecretKey": "s"}, data, false)
		if imAsIntDefault(plain["cover_width"], -1) != 3 || imAsIntDefault(plain["cover_height"], -1) != 2 {
			t.Errorf("plain cover = %v x %v, want 3x2", plain["cover_width"], plain["cover_height"])
		}
	})

	t.Run("assorted scalars", func(t *testing.T) {
		if got := imFallbackStr("", "def"); got != "def" {
			t.Errorf("imFallbackStr = %q", got)
		}
		if got := imFallbackStr("v", "def"); got != "v" {
			t.Errorf("imFallbackStr = %q", got)
		}
		ts := int64(1234)
		if got := imShareID("u", "i", &ts); got != "u_1234_i" {
			t.Errorf("imShareID = %q", got)
		}
		if got := imShareID("", "i", nil); got != "" {
			t.Errorf("imShareID(no uid) = %q, want empty", got)
		}
		if got := imShareID("u", "", nil); got != "" {
			t.Errorf("imShareID(no item) = %q, want empty", got)
		}
		for _, tc := range []struct {
			in   any
			want bool
		}{{true, true}, {false, false}, {nil, false}, {float64(2), true}, {float64(0), false}, {"1", true}, {"0", false}} {
			if got := imBool(tc.in); got != tc.want {
				t.Errorf("imBool(%v) = %v, want %v", tc.in, got, tc.want)
			}
		}
		if got := imAIExt(map[string]any{}); got != "{}" {
			t.Errorf("imAIExt(missing) = %q", got)
		}
		if got := imAIExt(map[string]any{"ai_ext": map[string]any{"b": 1, "a": 2}}); got != `{"a":2,"b":1}` {
			t.Errorf("imAIExt(map) = %q, want sorted JSON", got)
		}
		if got := imAIExt(map[string]any{"ai_ext": "raw"}); got != "raw" {
			t.Errorf("imAIExt(string) = %q", got)
		}
		if got := imAIExt(map[string]any{"ai_ext": []any{1, 2}}); got != "[1,2]" {
			t.Errorf("imAIExt(list) = %q", got)
		}
		if got := imDetailString(map[string]any{"desc": "", "title": "t"}, "desc", "title"); got != "t" {
			t.Errorf("imDetailString = %q", got)
		}
		if got := imOrEmptyList(nil); len(got.([]any)) != 0 {
			t.Errorf("imOrEmptyList(nil) = %v", got)
		}
		if got := imOrEmptyMap(nil); len(got.(map[string]any)) != 0 {
			t.Errorf("imOrEmptyMap(nil) = %v", got)
		}
		if got := imMapGetFirst(map[string]any{"a": nil, "b": "v"}, "a", "b"); got != "v" {
			t.Errorf("imMapGetFirst = %v", got)
		}
	})

	t.Run("item id extraction", func(t *testing.T) {
		for _, tc := range []struct{ in, want string }{
			{"7123456789", "7123456789"},
			{"  7123456789  ", "7123456789"},
			{"https://www.douyin.com/video/7123456789", "7123456789"},
			{"https://www.douyin.com/note/7123456789", "7123456789"},
			{"https://www.douyin.com/?modal_id=7123456789", "7123456789"},
			{"", ""},
			{"abc", ""},
		} {
			if got := imItemIDFromValue(tc.in); got != tc.want {
				t.Errorf("imItemIDFromValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})

	t.Run("url object", func(t *testing.T) {
		obj := imURLObject(map[string]any{"url_list": "u1"}, 10, 20, nil)
		if obj["uri"] != "u1" || obj["width"] != int64(10) || obj["height"] != int64(20) {
			t.Errorf("imURLObject(map) = %v", obj)
		}
		if urls, ok := obj["url_list"].([]any); !ok || len(urls) != 1 || urls[0] != "u1" {
			t.Errorf("url_list = %v", obj["url_list"])
		}
		// Existing dimensions win over the defaults.
		kept := imURLObject(map[string]any{"uri": "u", "width": 5, "height": 6}, 10, 20, nil)
		if kept["width"] != 5 || kept["height"] != 6 {
			t.Errorf("kept dimensions = %v x %v", kept["width"], kept["height"])
		}
		size := int64(7)
		sized := imURLObject("u", 0, 0, &size)
		if sized["data_size"] != int64(7) {
			t.Errorf("data_size = %v", sized["data_size"])
		}
		existing := imURLObject(map[string]any{"uri": "u", "data_size": 1}, 0, 0, &size)
		if existing["data_size"] != 1 {
			t.Errorf("existing data_size overwritten: %v", existing["data_size"])
		}
		first := imURLObject([]any{"a", "b"}, 0, 0, nil)
		if first["uri"] != "a" {
			t.Errorf("list object uri = %v, want a", first["uri"])
		}
		empty := imURLObject([]any{}, 0, 0, nil)
		if empty["uri"] != "" {
			t.Errorf("empty list uri = %v", empty["uri"])
		}
	})

	t.Run("author values", func(t *testing.T) {
		uid, sec, name := imAuthorValues(map[string]any{
			"author": map[string]any{"uid": "42", "sec_uid": "SEC", "nickname": "N"},
		})
		if uid != "42" || sec != "SEC" || name != "N" {
			t.Errorf("imAuthorValues = (%q,%q,%q)", uid, sec, name)
		}
		uid, sec, name = imAuthorValues(map[string]any{
			"uid": "7", "secUID": "S7", "content_name": "C",
		})
		if uid != "7" || sec != "S7" || name != "C" {
			t.Errorf("imAuthorValues fallback = (%q,%q,%q)", uid, sec, name)
		}
	})
}

func TestImMediaShareCardBuilders(t *testing.T) {
	detail := map[string]any{
		"author":           map[string]any{"uid": "42", "sec_uid": "SEC", "nickname": "N"},
		"desc":             "标题",
		"video":            map[string]any{"cover": map[string]any{"url_list": []any{"cover-url"}}, "width": 720, "height": 1280},
		"is_aigc":          true,
		"share_info":       []any{map[string]any{"k": 1}},
		"anchor_info":      map[string]any{"a": 1},
		"poi_track_params": nil,
		"send_source":      3,
		"profile_uid":      "p",
	}
	card := imBuildShareAwemeCard("item1", "", detail)
	if imAsIntDefault(card["aweType"], -1) != 800 || card["itemId"] != "item1" || card["uid"] != "42" ||
		card["secUID"] != "SEC" || card["content_name"] != "N" || card["content_title"] != "标题" {
		t.Fatalf("card = %v", card)
	}
	cover, _ := card["cover_url"].(map[string]any)
	if cover["uri"] != "cover-url" {
		t.Errorf("cover = %v", cover)
	}
	if imAsIntDefault(card["cover_width"], -1) != 720 || imAsIntDefault(card["cover_height"], -1) != 1280 {
		t.Errorf("cover size = %v x %v", card["cover_width"], card["cover_height"])
	}
	if card["is_aigc"] != true {
		t.Errorf("is_aigc = %v", card["is_aigc"])
	}
	if list, ok := card["share_info"].([]any); !ok || len(list) != 1 {
		t.Errorf("share_info = %v", card["share_info"])
	}
	if m, ok := card["poi_track_params"].(map[string]any); !ok || len(m) != 0 {
		t.Errorf("poi_track_params = %v, want empty map", card["poi_track_params"])
	}
	if card["ai_ext"] != "{}" {
		t.Errorf("ai_ext = %v", card["ai_ext"])
	}
	if card["send_source"] != 3 || card["profile_uid"] != "p" {
		t.Errorf("extras not copied: %v", card)
	}
	if share, _ := card["share_id"].(string); !strings.HasPrefix(share, "42_") || !strings.HasSuffix(share, "_item1") {
		t.Errorf("share_id = %q", share)
	}

	// Photos card: cover comes from images[0], image_count defaults to len(images).
	photos := map[string]any{
		"images": []any{map[string]any{
			"display_image": map[string]any{"url_list": []any{"img-1"}},
			"width":         5,
			"height":        6,
		}},
	}
	pc := imBuildSharePhotosCard("item2", "9", photos)
	if imAsIntDefault(pc["awemeType"], -1) != 68 || imAsIntDefault(pc["aweType"], -1) != 0 {
		t.Errorf("photos types = %v/%v", pc["awemeType"], pc["aweType"])
	}
	if imAsIntDefault(pc["image_count"], -1) != 1 || pc["uid"] != "9" {
		t.Errorf("image_count/uid = %v/%v", pc["image_count"], pc["uid"])
	}
	if imAsIntDefault(pc["cover_width"], -1) != 5 || imAsIntDefault(pc["cover_height"], -1) != 6 {
		t.Errorf("photos cover size = %v x %v", pc["cover_width"], pc["cover_height"])
	}
	if _, ok := pc["cover_url_v2"]; !ok {
		t.Error("cover_url_v2 missing")
	}
	// No images at all: still a valid card with count 1.
	empty := imBuildSharePhotosCard("item3", "9", map[string]any{})
	if imAsIntDefault(empty["image_count"], -1) != 1 {
		t.Errorf("empty image_count = %v, want 1", empty["image_count"])
	}
	cover, _, _ = imDetailCover(map[string]any{}, true)
	if cover["uri"] != "" {
		t.Errorf("empty photos cover = %v", cover)
	}

	// Video cover fallback: detail-level cover_url string.
	cover, width, height := imDetailCover(map[string]any{
		"cover_url": "c-url",
		"video":     map[string]any{"width": 4, "height": 5},
	}, false)
	if cover["uri"] != "c-url" || width != 4 || height != 5 {
		t.Errorf("video cover = (%v, %d, %d)", cover, width, height)
	}

	t.Run("web card", func(t *testing.T) {
		web := imBuildShareWebCard("https://www.douyin.com/video/1", "t", "d", "c")
		link, _ := web["link_url"].(string)
		if !strings.Contains(link, "pc_iframe_src=") {
			t.Fatalf("link_url = %q", link)
		}
		q, err := url.ParseQuery(strings.SplitN(link, "?", 2)[1])
		if err != nil {
			t.Fatalf("ParseQuery: %v", err)
		}
		if q.Get("pc_iframe_src") != "https://www.douyin.com/video/1" {
			t.Errorf("pc_iframe_src = %q", q.Get("pc_iframe_src"))
		}
		// An existing pc_iframe_src is preserved.
		again := imBuildShareWebCard("https://x/?pc_iframe_src=keep", "t", "d", "c")
		if got, _ := again["link_url"].(string); !strings.Contains(got, "pc_iframe_src=keep") {
			t.Errorf("link_url = %q", got)
		}
		if empty := imBuildShareWebCard("", "t", "d", "c"); empty["link_url"] != "" {
			t.Errorf("empty link_url = %v", empty["link_url"])
		}
	})

	t.Run("user card", func(t *testing.T) {
		uc := imBuildUserCard("1", "s", "n", "https://a/x.png", []any{"u1", "u2"})
		if uc["uid"] != "1" || uc["secUID"] != "s" || uc["name"] != "n" {
			t.Errorf("user card = %v", uc)
		}
		avatar, _ := uc["avatar"].(map[string]any)
		if avatar["uri"] != "https://a/x.png" {
			t.Errorf("avatar = %v", avatar)
		}
		covers, _ := uc["cover_url"].([]any)
		if len(covers) != 2 {
			t.Fatalf("cover_url = %v, want 2 entries", covers)
		}
		if items, ok := uc["cover_items"].([]any); !ok || len(items) != 0 {
			t.Errorf("cover_items = %v", uc["cover_items"])
		}
		single := imBuildUserCard("1", "s", "n", nil, "only")
		if covers, _ := single["cover_url"].([]any); len(covers) != 1 {
			t.Errorf("scalar cover_items = %v, want 1 entry", single["cover_url"])
		}
	})
}

// ---------------------------------------------------------------------------
// im_ws.go
// ---------------------------------------------------------------------------

func TestImWSFrameDecoding(t *testing.T) {
	newReceiver := func() *IMReceiver {
		r := &IMReceiver{ch: make(chan IMMessage, 4), stop: make(chan struct{})}
		r.initStop()
		return r
	}
	drain := func(t *testing.T, r *IMReceiver) (IMMessage, bool) {
		t.Helper()
		select {
		case msg := <-r.ch:
			return msg, true
		default:
			return IMMessage{}, false
		}
	}

	t.Run("valid frame", func(t *testing.T) {
		frame := imMediaTestNotifyFrame(1, 1)
		r := newReceiver()
		r.handleFrame(frame)
		msg, ok := drain(t, r)
		if !ok {
			t.Fatal("no message emitted")
		}
		if msg.MessageIndex != 77 || msg.ConversationID != "0:1:111:222" || msg.ConversationType != 1 ||
			msg.NotifyType != 50001 || msg.Sender != 111 || msg.MessageType != 7 {
			t.Fatalf("message = %+v", msg)
		}
		if msg.Content["text"] != "hi" {
			t.Errorf("content = %v", msg.Content)
		}
	})

	t.Run("conversation type fallback", func(t *testing.T) {
		r := newReceiver()
		r.handleFrame(imMediaTestNotifyFrame(0, 2))
		msg, ok := drain(t, r)
		if !ok {
			t.Fatal("no message emitted")
		}
		if msg.ConversationType != 2 {
			t.Errorf("conversation type = %d, want 2 from the message body", msg.ConversationType)
		}
	})

	t.Run("non pb payload type", func(t *testing.T) {
		r := newReceiver()
		r.handleFrame(imMediaTestPushFrame("ack", imMediaTestNotifyFrame(1, 1)))
		if _, ok := drain(t, r); ok {
			t.Fatal("frames with payloadType != pb must be dropped")
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		r := newReceiver()
		for _, frame := range [][]byte{
			imMediaTestPushFrame("pb", nil),
			imMediaTestPushFrame("hb", nil),
			imMediaTestPushFrame("pb", []byte{}),
		} {
			r.handleFrame(frame)
		}
		if _, ok := drain(t, r); ok {
			t.Fatal("empty payloads must not emit a message")
		}
	})

	t.Run("response without body", func(t *testing.T) {
		resp := &pbw{}
		resp.IntAlways(1, 500)
		r := newReceiver()
		r.handleFrame(imMediaTestPushFrame("pb", resp.b))
		if _, ok := drain(t, r); ok {
			t.Fatal("a Response without body must not emit a message")
		}
	})

	t.Run("invalid response payload", func(t *testing.T) {
		r := newReceiver()
		r.handleFrame(imMediaTestPushFrame("pb", []byte{0xff}))
		if _, ok := drain(t, r); ok {
			t.Fatal("undecodable Response payload must be dropped")
		}
	})

	t.Run("truncated frame", func(t *testing.T) {
		// field 8 (payload), declared length 10, no data behind it.
		trunc := []byte{0x42, 0x0a}
		if _, err := ProtoUnmarshal("PushFrame", trunc); err == nil {
			t.Fatal("truncated PushFrame must fail to decode")
		}
		r := newReceiver()
		r.handleFrame(trunc)
		if _, ok := drain(t, r); ok {
			t.Fatal("truncated frames must be dropped")
		}
		r.handleFrame(nil)
		if _, ok := drain(t, r); ok {
			t.Fatal("nil frame must be dropped")
		}
	})

	t.Run("proto accessors", func(t *testing.T) {
		fm, err := ProtoUnmarshal("PushFrame", imMediaTestPushFrame("pb", []byte{0x01}))
		if err != nil {
			t.Fatalf("ProtoUnmarshal: %v", err)
		}
		m := fm.ProtoReflect()
		if got := protoGetString(m, "payloadType"); got != "pb" {
			t.Errorf("protoGetString = %q, want pb", got)
		}
		if got := protoGetString(m, "missing"); got != "" {
			t.Errorf("protoGetString(missing) = %q", got)
		}
		if _, ok := protoSub(m, "payload"); ok {
			t.Error("protoSub must reject non-message (bytes) fields")
		}
		if _, ok := protoSub(m, "missing"); ok {
			t.Error("protoSub must reject unknown fields")
		}
		if _, ok := protoSub(nil, "payload"); ok {
			t.Error("protoSub(nil) must be false")
		}

		// protoGetInt64 only accepts the signed integer kinds: drive it off
		// Response's int32 cmd / int64 sequence_id.
		resp := &pbw{}
		resp.IntAlways(1, 500)
		resp.IntAlways(2, 7)
		rm, err := ProtoUnmarshal("Response", resp.b)
		if err != nil {
			t.Fatalf("ProtoUnmarshal(Response): %v", err)
		}
		rmr := rm.ProtoReflect()
		if got := protoGetInt64(rmr, "sequence_id"); got != 7 {
			t.Errorf("protoGetInt64(sequence_id) = %d, want 7", got)
		}
		if got := protoGetInt64(rmr, "cmd"); got != 500 {
			t.Errorf("protoGetInt64(cmd) = %d, want 500", got)
		}
		if got := protoGetInt64(rmr, "missing"); got != 0 {
			t.Errorf("protoGetInt64(missing) = %d, want 0", got)
		}
	})
}

func TestImWSReceiverLifecycleAndBackoff(t *testing.T) {
	if _, err := NewIMReceiver(nil); err == nil {
		t.Fatal("NewIMReceiver(nil) must error")
	}

	c, _ := newStubClient(t, nil)
	c.SetWebID("dev-x")
	r, err := NewIMReceiver(c)
	if err != nil {
		t.Fatalf("NewIMReceiver: %v", err)
	}
	if !strings.HasPrefix(r.url, imWSURL+"?") {
		t.Fatalf("url = %q, want %q prefix", r.url, imWSURL+"?")
	}
	q := imMediaTestQuery(t, r.url)
	wantAccess := MD5Hex(imFpID + imAppKey + "dev-x" + imWSSalt)
	for k, want := range map[string]string{
		"aid":             "6383",
		"device_platform": "douyin_pc",
		"fpid":            imFpID,
		"device_id":       "dev-x",
		"token":           "abc", // harness cookie: sessionid=abc
		"access_key":      wantAccess,
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}
	if r.Events() == nil {
		t.Fatal("Events() must return the message channel")
	}
	if r.stopped() {
		t.Fatal("fresh receiver reports stopped")
	}

	// A cancelled context returns before any dial is attempted.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start(cancelled) = %v, want context.Canceled", err)
	}

	// Stop is idempotent and makes Start return nil instead of dialing.
	r.Stop()
	r.Stop()
	if !r.stopped() {
		t.Fatal("Stop() did not mark the receiver stopped")
	}
	if err := r.Start(t.Context()); err != nil {
		t.Fatalf("Start after Stop = %v, want nil", err)
	}

	// The reconnect loop doubles a seconds-based delay from imBackoffStart and
	// clamps it at imBackoffMax. The loop itself cannot be driven offline (it
	// dials frontier-im and sleeps in whole seconds), so pin the documented
	// schedule it implements: 1s, 2s, 4s, 8s, 16s, 30s, 30s…
	if imBackoffStart != 1.0 || imBackoffMax != 30.0 {
		t.Fatalf("backoff bounds = (%v, %v), want (1s, 30s)", imBackoffStart, imBackoffMax)
	}
	var got []float64
	backoff := imBackoffStart
	for range 7 {
		got = append(got, backoff)
		backoff *= 2
		if backoff > imBackoffMax {
			backoff = imBackoffMax
		}
	}
	want := []float64{1, 2, 4, 8, 16, 30, 30}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("reconnect schedule = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// improto.go
// ---------------------------------------------------------------------------

func TestImProtoWriterReaderRoundTrip(t *testing.T) {
	w := &pbw{}
	w.Int(1, 0)       // zero is skipped
	w.IntAlways(2, 0) // zero is written
	w.Int(3, 300)
	w.Str(4, "") // empty is skipped
	w.Str(5, "hi")
	w.Bytes(6, nil) // empty is skipped
	w.Bytes(7, []byte{0xaa})
	inner := &pbw{}
	inner.Str(1, "deep")
	w.Msg(8, inner)

	got, err := pbParse(w.b)
	if err != nil {
		t.Fatalf("pbParse: %v", err)
	}
	if _, ok := got[1]; ok {
		t.Error("Int(0) must not be written")
	}
	if v, ok := got[2].(int64); !ok || v != 0 {
		t.Errorf("IntAlways(0) = %#v, want int64(0)", got[2])
	}
	if v, ok := got[3].(int64); !ok || v != 300 {
		t.Errorf("field 3 = %#v, want 300", got[3])
	}
	if _, ok := got[4]; ok {
		t.Error("Str(\"\") must not be written")
	}
	if v, ok := got[5].([]byte); !ok || string(v) != "hi" {
		t.Errorf("field 5 = %#v, want []byte(hi)", got[5])
	}
	if _, ok := got[6]; ok {
		t.Error("Bytes(nil) must not be written")
	}
	if v, ok := got[7].([]byte); !ok || len(v) != 1 || v[0] != 0xaa {
		t.Errorf("field 7 = %#v", got[7])
	}
	nested, ok := pbMsg(got, 8)
	if !ok || pbString(nested, 1) != "deep" {
		t.Errorf("nested message = %v (%v)", nested, ok)
	}

	// Exact wire bytes for the low-level writer.
	wire := func(build func(*pbw)) []byte {
		x := &pbw{}
		build(x)
		return x.b
	}
	if got := wire(func(x *pbw) { x.IntAlways(1, 0) }); !bytes.Equal(got, []byte{0x08, 0x00}) {
		t.Errorf("IntAlways(1,0) wire bytes = % x, want 08 00", got)
	}
	if got := wire(func(x *pbw) { x.Str(2, "hi") }); !bytes.Equal(got, []byte{0x12, 0x02, 'h', 'i'}) {
		t.Errorf("Str wire bytes = % x", got)
	}
	if got := wire(func(x *pbw) { x.Bytes(3, []byte{0xaa}) }); !bytes.Equal(got, []byte{0x1a, 0x01, 0xaa}) {
		t.Errorf("Bytes wire bytes = % x", got)
	}
	if got := wire(func(x *pbw) { x.Int(4, 300) }); !bytes.Equal(got, []byte{0x20, 0xac, 0x02}) {
		t.Errorf("Int(300) wire bytes = % x", got)
	}
}

func TestImProtoWireTypesAndVarints(t *testing.T) {
	// Wire type 1 (fixed64, little endian).
	fixed64 := []byte{0x09, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	m, err := pbParse(fixed64)
	if err != nil {
		t.Fatalf("pbParse(fixed64): %v", err)
	}
	if v, ok := m[1].(uint64); !ok || v != 0x0807060504030201 {
		t.Errorf("fixed64 = %#v, want uint64(0x0807060504030201)", m[1])
	}

	// Wire type 5 (fixed32, little endian).
	m, err = pbParse([]byte{0x15, 0x01, 0x02, 0x03, 0x04})
	if err != nil {
		t.Fatalf("pbParse(fixed32): %v", err)
	}
	if v, ok := m[2].(uint32); !ok || v != 0x04030201 {
		t.Errorf("fixed32 = %#v, want uint32(0x04030201)", m[2])
	}

	// Wire type 2 with a zero length.
	m, err = pbParse([]byte{0x1a, 0x00})
	if err != nil {
		t.Fatalf("pbParse(empty bytes): %v", err)
	}
	if v, ok := m[3].([]byte); !ok || len(v) != 0 {
		t.Errorf("empty bytes = %#v", m[3])
	}

	// Repeated fields accumulate in order.
	repeated := &pbw{}
	repeated.IntAlways(7, 1)
	repeated.IntAlways(7, 2)
	repeated.IntAlways(7, 3)
	m, err = pbParse(repeated.b)
	if err != nil {
		t.Fatalf("pbParse(repeated): %v", err)
	}
	list, ok := m[7].([]any)
	if !ok || len(list) != 3 || list[0].(int64) != 1 || list[1].(int64) != 2 || list[2].(int64) != 3 {
		t.Fatalf("repeated field = %#v", m[7])
	}

	// Varint boundaries (pbw.varint byte-for-byte).
	for _, tc := range []struct {
		v    uint64
		want []byte
	}{
		{0, []byte{0x00}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{300, []byte{0xac, 0x02}},
		{16383, []byte{0xff, 0x7f}},
		{16384, []byte{0x80, 0x80, 0x01}},
		{^uint64(0), []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
	} {
		w := &pbw{}
		w.varint(tc.v)
		if !bytes.Equal(w.b, tc.want) {
			t.Errorf("varint(%d) = % x, want % x", tc.v, w.b, tc.want)
		}
	}

	// Round trip every boundary through pbParse (int64 reinterpretation).
	for _, v := range []uint64{0, 127, 128, 16383, 16384, ^uint64(0)} {
		w := &pbw{}
		w.IntAlways(1, int64(v))
		m, err := pbParse(w.b)
		if err != nil {
			t.Fatalf("pbParse(varint %d): %v", v, err)
		}
		if got, ok := m[1].(int64); !ok || got != int64(v) {
			t.Errorf("varint round trip %d = %#v", v, m[1])
		}
	}

	// Accessors.
	acc := map[int]any{1: int64(42), 2: []byte("s"), 3: uint32(9), 4: uint64(10), 5: float64(1.5), 6: int(7)}
	if got := pbString(acc, 1); got != "42" {
		t.Errorf("pbString(int64) = %q", got)
	}
	if got := pbString(acc, 2); got != "s" {
		t.Errorf("pbString(bytes) = %q", got)
	}
	if got := pbString(acc, 99); got != "" {
		t.Errorf("pbString(missing) = %q", got)
	}
	if got := pbInt(acc, 3); got != 9 {
		t.Errorf("pbInt(uint32) = %d", got)
	}
	if got := pbInt(acc, 4); got != 10 {
		t.Errorf("pbInt(uint64) = %d", got)
	}
	if got := pbInt(acc, 1); got != 42 {
		t.Errorf("pbInt(int64) = %d", got)
	}
	if got := pbInt(acc, 99); got != 0 {
		t.Errorf("pbInt(missing) = %d", got)
	}
	if got := pbInt(map[int]any{1: []byte("17")}, 1); got != 17 {
		t.Errorf("pbInt(ascii bytes) = %d", got)
	}
	if got := pbList(acc, 99); got != nil {
		t.Errorf("pbList(nil) = %v", got)
	}
	if got := pbList(acc, 1); len(got) != 1 || got[0].(int64) != 42 {
		t.Errorf("pbList(single) = %v", got)
	}
	if got := pbList(map[int]any{1: []any{1, 2}}, 1); len(got) != 2 {
		t.Errorf("pbList(list) = %v", got)
	}
	if got := pbF(acc[5]); got != 1.5 {
		t.Errorf("pbF(float64) = %v", got)
	}
	if got := pbF(acc[6]); got != 7 {
		t.Errorf("pbF(int) = %v", got)
	}
	if got := pbF(int64(3)); got != 3 {
		t.Errorf("pbF(int64) = %v", got)
	}
	if got := pbF(acc[99]); !mathIsNaN(got) {
		t.Errorf("pbF(missing) = %v, want NaN", got)
	}
	if got := pbF("not a number"); !mathIsNaN(got) {
		t.Errorf("pbF(string) = %v, want NaN", got)
	}
	if _, ok := pbMsg(map[int]any{1: []byte{0xff}}, 1); ok {
		t.Error("pbMsg must reject unparseable nested bytes")
	}
	if _, ok := pbMsg(map[int]any{1: int64(1)}, 1); ok {
		t.Error("pbMsg must reject non-bytes fields")
	}
}

// mathIsNaN avoids importing math just for one assertion.
func mathIsNaN(f float64) bool { return f != f }

func TestImProtoMalformedInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"bad tag", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01, 0x00}},
		{"truncated varint", []byte{0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}},
		{"varint missing", []byte{0x08}},
		{"length past end", []byte{0x0a, 0x05, 'a'}},
		{"short fixed64", []byte{0x09, 0x01, 0x02}},
		{"short fixed32", []byte{0x0d, 0x01}},
		{"group wire type", []byte{0x0b, 0x00}},
		{"unknown wire type", []byte{0x0e, 0x00}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := pbParse(tc.in)
			if err == nil {
				t.Fatalf("pbParse(% x) = %v, want error", tc.in, m)
			}
			if m != nil {
				t.Errorf("pbParse returned a map alongside the error: %v", m)
			}
		})
	}

	m, err := pbParse(nil)
	if err != nil || len(m) != 0 {
		t.Fatalf("pbParse(nil) = (%v, %v), want empty map", m, err)
	}
}

func TestImProtoEnvelopeAndResponse(t *testing.T) {
	entries := imHeaderEntries()
	if len(entries) != 17 {
		t.Fatalf("imHeaderEntries = %d entries, want 17", len(entries))
	}
	if entries[0] != [2]string{"session_aid", "6383"} || entries[1] != [2]string{"session_did", "0"} ||
		entries[len(entries)-1] != [2]string{"is-retry", "0"} {
		t.Errorf("header entries = %v", entries)
	}
	found := false
	for _, e := range entries {
		if e == [2]string{"browser_platform", "Win32"} {
			found = true
		}
	}
	if !found {
		t.Error("browser_platform=Win32 entry missing")
	}

	inner := &pbw{}
	inner.Str(1, "inner-value")
	body := &pbw{}
	body.Msg(2048, inner) // imCall wraps the cmd-specific body under its cmd id
	env, err := pbParse(imEnvelope(2048, 1, body))
	if err != nil {
		t.Fatalf("pbParse(envelope): %v", err)
	}
	if pbInt(env, 1) != 2048 || pbInt(env, 6) != 1 {
		t.Errorf("cmd/inbox = %d/%d, want 2048/1", pbInt(env, 1), pbInt(env, 6))
	}
	if pbString(env, 3) != imSDKVersion || pbString(env, 7) != imBuildNumber {
		t.Errorf("sdk/build = %q/%q", pbString(env, 3), pbString(env, 7))
	}
	if pbString(env, 11) != imAPIPlatform || pbString(env, 21) != "douyin_web" || pbString(env, 22) != "web_sdk" {
		t.Errorf("platform fields = %q/%q/%q", pbString(env, 11), pbString(env, 21), pbString(env, 22))
	}
	if pbInt(env, 5) != 3 || pbInt(env, 18) != 1 {
		t.Errorf("refer/auth = %d/%d, want 3/1", pbInt(env, 5), pbInt(env, 18))
	}
	if seq := pbInt(env, 2); seq < 10000 || seq > 10999 {
		t.Errorf("sequence_id = %d, want 10000..10999", seq)
	}
	wrapped, ok := pbMsg(env, 8)
	if !ok {
		t.Fatal("envelope body (field 8) missing")
	}
	sent, ok := pbMsg(wrapped, 2048)
	if !ok || pbString(sent, 1) != "inner-value" {
		t.Errorf("body wrapper = %v (%v)", sent, ok)
	}
	if headers := pbList(env, 15); len(headers) != 17 {
		t.Fatalf("header count = %d, want 17", len(headers))
	}
	first, ok := pbMsg(map[int]any{15: pbList(env, 15)[0]}, 15)
	if !ok || pbString(first, 1) != "session_aid" || pbString(first, 2) != "6383" {
		t.Errorf("first header = %v (%v)", first, ok)
	}

	// Valid Response envelope.
	cmdBody := &pbw{}
	cmdBody.Str(1, "payload")
	bodyMsg := &pbw{}
	bodyMsg.Msg(2048, cmdBody)
	responseEnv := &pbw{}
	responseEnv.IntAlways(1, 2048)
	responseEnv.Bytes(6, bodyMsg.b)

	got, code, msg, err := imResponseBody(responseEnv.b)
	if err != nil || code != 0 || msg != "" || got == nil {
		t.Fatalf("imResponseBody = (%v, %d, %q, %v)", got, code, msg, err)
	}
	if inner, ok := pbMsg(got, 2048); !ok || pbString(inner, 1) != "payload" {
		t.Errorf("cmd body = %v (%v)", inner, ok)
	}
	if cmd := imCmdBody(got, 2048); pbString(cmd, 1) != "payload" {
		t.Errorf("imCmdBody = %v", cmd)
	}
	if cmd := imCmdBody(got, 999); cmd != nil {
		t.Errorf("imCmdBody(missing) = %v, want nil", cmd)
	}

	// Error envelope: no body, code + message carried through.
	errEnv := &pbw{}
	errEnv.IntAlways(3, 7)
	errEnv.Str(4, "boom")
	got, code, msg, err = imResponseBody(errEnv.b)
	if err != nil || code != 7 || msg != "boom" || got != nil {
		t.Fatalf("error envelope = (%v, %d, %q, %v)", got, code, msg, err)
	}

	// Success code without a body is malformed.
	soloEnv := &pbw{}
	soloEnv.IntAlways(1, 2048)
	if _, _, _, err := imResponseBody(soloEnv.b); err == nil ||
		!strings.Contains(err.Error(), "no body") {
		t.Fatalf("missing body err = %v", err)
	}
	if _, _, _, err := imResponseBody([]byte{0x08}); err == nil {
		t.Fatal("truncated envelope must error")
	}
	badBody := &pbw{}
	badBody.IntAlways(1, 2048)
	badBody.Bytes(6, []byte{0x0a, 0x05, 'a'})
	if _, _, _, err := imResponseBody(badBody.b); err == nil {
		t.Fatal("unparseable body must error")
	}
}

// ---------------------------------------------------------------------------
// api_misc_im.go
// ---------------------------------------------------------------------------

func TestMiscImJSONStringArray(t *testing.T) {
	for _, tc := range []struct {
		in   []string
		want string
	}{
		{nil, "[]"},
		{[]string{}, "[]"},
		{[]string{"a", "b"}, `["a","b"]`},
		{[]string{`a"b`}, `["a\"b"]`},
		{[]string{"<x>"}, `["\u003cx\u003e"]`},
	} {
		if got := imJSONStringArray(tc.in); got != tc.want {
			t.Errorf("imJSONStringArray(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMiscImRequestShapes(t *testing.T) {
	ctx := t.Context()

	t.Run("spotlight relation", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		got, err := c.IMGetSpotlightRelation(ctx, "10", "99")
		if err != nil {
			t.Fatalf("IMGetSpotlightRelation: %v", err)
		}
		if toInt64(got["status_code"]) != 0 {
			t.Errorf("payload = %v", got)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, imHJBase+"/aweme/v1/web/im/spotlight/relation/") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		for k, want := range map[string]string{
			"count":                   "10",
			"max_time":                "99",
			"min_time":                "0",
			"need_remove_share_panel": "true",
			"need_sorted_info":        "true",
			"with_fstatus":            "1",
			"device_platform":         "webapp",
			"aid":                     "6383",
			"webid":                   "webid-test",
			"msToken":                 "ms-token-test",
			"uifid":                   "uif",
		} {
			if got := q.Get(k); got != want {
				t.Errorf("query %s = %q, want %q", k, got, want)
			}
		}
		if q.Get("verifyFp") == "" || q.Get("fp") == "" || q.Get("a_bogus") == "" {
			t.Errorf("signature params missing: %v", q)
		}
		if got := req.Header("referer"); got != douyinBase+"/friend" {
			t.Errorf("referer = %q", got)
		}

		// Defaults when both arguments are empty.
		c2, st2 := imMediaTestNewMediaClient(t, nil)
		if _, err := c2.IMGetSpotlightRelation(ctx, "", ""); err != nil {
			t.Fatalf("IMGetSpotlightRelation(defaults): %v", err)
		}
		q2 := imMediaTestQuery(t, st2.last(t).URL)
		if q2.Get("count") != "50" || q2.Get("max_time") != "0" {
			t.Errorf("defaults = count %q max_time %q", q2.Get("count"), q2.Get("max_time"))
		}
	})

	t.Run("active status", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetActiveStatus(ctx, []string{"c1", "c2"}, nil); err != nil {
			t.Fatalf("IMGetActiveStatus: %v", err)
		}
		req := st.last(t)
		if req.Method != "POST" || !strings.HasPrefix(req.URL, imHJBase+"/aweme/v1/web/im/user/active/status/") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		form, err := url.ParseQuery(string(req.Body))
		if err != nil {
			t.Fatalf("ParseQuery(body): %v", err)
		}
		if got := form.Get("conv_ids"); got != `["c1","c2"]` {
			t.Errorf("conv_ids = %q", got)
		}
		if got := form.Get("sec_user_ids"); got != "[]" {
			t.Errorf("sec_user_ids = %q, want []", got)
		}
		if got := form.Get("source"); got != "heartbeat" {
			t.Errorf("source = %q", got)
		}
		if got := imMediaTestQuery(t, req.URL).Get("webid"); got != "webid-test" {
			t.Errorf("webid = %q", got)
		}
	})

	t.Run("active heartbeat", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMActiveHeartbeat(ctx, ""); err != nil {
			t.Fatalf("IMActiveHeartbeat: %v", err)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, imHJBase+"/aweme/v1/web/im/user/active/update/") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		if q.Get("action") != "heartbeat" || q.Get("new_user_login") != "0" {
			t.Errorf("query = %v", q)
		}
		c2, st2 := imMediaTestNewMediaClient(t, nil)
		if _, err := c2.IMActiveHeartbeat(ctx, "1"); err != nil {
			t.Fatalf("IMActiveHeartbeat(1): %v", err)
		}
		if got := imMediaTestQuery(t, st2.last(t).URL).Get("new_user_login"); got != "1" {
			t.Errorf("new_user_login = %q", got)
		}
	})

	t.Run("active config", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetActiveConfig(ctx); err != nil {
			t.Fatalf("IMGetActiveConfig: %v", err)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, imHJBase+"/aweme/v1/web/im/user/active/config/get") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
	})

	t.Run("strategy config", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetStrategyConfig(ctx, ""); err != nil {
			t.Fatalf("IMGetStrategyConfig: %v", err)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, douyinBase+"/aweme/v1/web/im/strategy/config") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		if q.Get("app_id") != "1128" || q.Get("scenes") != `["interactive_resources"]` {
			t.Errorf("query = %v", q)
		}
	})

	t.Run("resources", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetResources(ctx, "", "", ""); err != nil {
			t.Fatalf("IMGetResources: %v", err)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, imHJBase+"/aweme/v1/web/im/resource/list/aggregation/") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		for k, want := range map[string]string{
			"app_id": "1128", "scenes": "CUSTOM_STICKER_PAGE", "custom_cursor": "0", "custom_limit": "50",
		} {
			if got := q.Get(k); got != want {
				t.Errorf("query %s = %q, want %q", k, got, want)
			}
		}
	})

	t.Run("emoticon trending", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetEmoticonTrending(ctx, "", "", ""); err != nil {
			t.Fatalf("IMGetEmoticonTrending: %v", err)
		}
		req := st.last(t)
		if req.Method != "GET" || !strings.HasPrefix(req.URL, douyinBase+"/aweme/v1/web/im/resources/emoticon/trending") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		for k, want := range map[string]string{"cursor": "0", "count": "50", "groupId": "1"} {
			if got := q.Get(k); got != want {
				t.Errorf("query %s = %q, want %q", k, got, want)
			}
		}
	})

	t.Run("feedback entrance", func(t *testing.T) {
		c, st := imMediaTestNewMediaClient(t, nil)
		if _, err := c.IMGetFeedbackEntrance(ctx, ""); err != nil {
			t.Fatalf("IMGetFeedbackEntrance: %v", err)
		}
		req := st.last(t)
		if req.Method != "POST" || !strings.HasPrefix(req.URL, douyinBase+"/aweme/v1/web/im/get/online_feedback/entrance/") {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		q := imMediaTestQuery(t, req.URL)
		if q.Get("app_id") != "10001" || q.Get("entrance") != "IM6383-3586" {
			t.Errorf("query = %v", q)
		}
		if len(req.Body) != 0 {
			t.Errorf("body = %q, want empty", req.Body)
		}
	})
}

func TestMiscImResponseAndErrorMapping(t *testing.T) {
	ctx := t.Context()

	t.Run("numbers stay exact", func(t *testing.T) {
		c, _ := newStubJSON(t, map[string]any{"status_code": 0, "big": 7123456789012345678})
		got, err := c.IMGetSpotlightRelation(ctx, "", "")
		if err != nil {
			t.Fatalf("IMGetSpotlightRelation: %v", err)
		}
		if n, ok := got["big"].(json.Number); !ok || n.String() != "7123456789012345678" {
			t.Errorf("big = %#v, want an exact json.Number", got["big"])
		}
	})

	t.Run("empty body", func(t *testing.T) {
		c, _ := imMediaTestNewMediaClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, ""), nil
		})
		_, err := c.IMGetSpotlightRelation(ctx, "", "")
		if err == nil || !strings.Contains(err.Error(), "空响应") {
			t.Fatalf("err = %v, want 空响应", err)
		}
	})

	t.Run("non json body", func(t *testing.T) {
		c, _ := imMediaTestNewMediaClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "<html>challenge</html>"), nil
		})
		_, err := c.IMGetActiveStatus(ctx, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "非 JSON") {
			t.Fatalf("err = %v, want 非 JSON", err)
		}
	})

	t.Run("truncated json object", func(t *testing.T) {
		c, _ := imMediaTestNewMediaClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, `{"status_code":`), nil
		})
		if _, err := c.IMActiveHeartbeat(ctx, ""); err == nil {
			t.Fatal("truncated JSON must fail to decode")
		}
	})
}

func TestMiscImPullMessages(t *testing.T) {
	ctx := t.Context()

	inner := &pbw{}
	inner.IntAlways(2, 72313)
	inner.Bytes(3, []byte("t"))
	body := &pbw{}
	body.Msg(2048, inner)
	envelope := &pbw{}
	envelope.IntAlways(1, 2048)
	envelope.Bytes(6, body.b)

	t.Run("request shape and decoding", func(t *testing.T) {
		c, st := newStubClient(t, func(stubRequest) (*Response, error) {
			return &Response{StatusCode: 200, Body: envelope.b}, nil
		})
		got, err := c.IMPullMessages(ctx, 111, 222)
		if err != nil {
			t.Fatalf("IMPullMessages: %v", err)
		}
		if v, ok := got["2"].(int64); !ok || v != 72313 {
			t.Errorf("decoded field 2 = %#v, want int64(72313)", got["2"])
		}
		if got["3"] != "t" {
			t.Errorf("decoded field 3 = %#v, want \"t\"", got["3"])
		}

		req := st.last(t)
		if req.Method != "POST" || req.URL != imBase+"/v1/message/get_user_message" {
			t.Fatalf("request = %s %s", req.Method, req.URL)
		}
		if got := req.Header("content-type"); got != "application/x-protobuf" {
			t.Errorf("content-type = %q", got)
		}
		if got := req.Header("accept"); got != "application/x-protobuf" {
			t.Errorf("accept = %q", got)
		}
		if got := req.Header("referer"); got != "https://www.douyin.com/" {
			t.Errorf("referer = %q", got)
		}
		env, err := pbParse(req.Body)
		if err != nil {
			t.Fatalf("pbParse(request): %v", err)
		}
		if pbInt(env, 1) != 2048 || pbInt(env, 6) != 1 {
			t.Errorf("cmd/inbox = %d/%d, want 2048/1", pbInt(env, 1), pbInt(env, 6))
		}
		wrapped, ok := pbMsg(env, 8)
		if !ok {
			t.Fatal("request body wrapper (field 8) missing")
		}
		sent, ok := pbMsg(wrapped, 2048)
		if !ok {
			t.Fatal("cmd body (field 2048) missing")
		}
		if pbInt(sent, 1) != 111 || pbInt(sent, 2) != 72313 || pbInt(sent, 4) != 222 {
			t.Errorf("sent body = %v", sent)
		}
	})

	t.Run("error code", func(t *testing.T) {
		errEnv := &pbw{}
		errEnv.IntAlways(3, 42)
		errEnv.Str(4, "boom")
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return &Response{StatusCode: 200, Body: errEnv.b}, nil
		})
		_, err := c.IMPullMessages(ctx, 1, 2)
		if err == nil || !strings.Contains(err.Error(), "错误码 42") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want 错误码 42 boom", err)
		}
	})

	t.Run("undecodable response", func(t *testing.T) {
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return statusResponse(200, "\x08"), nil
		})
		_, err := c.IMPullMessages(ctx, 1, 2)
		if err == nil || !strings.Contains(err.Error(), "解析响应失败") {
			t.Fatalf("err = %v, want 解析响应失败", err)
		}
	})

	t.Run("missing body", func(t *testing.T) {
		solo := &pbw{}
		solo.IntAlways(1, 2048)
		c, _ := newStubClient(t, func(stubRequest) (*Response, error) {
			return &Response{StatusCode: 200, Body: solo.b}, nil
		})
		_, err := c.IMPullMessages(ctx, 1, 2)
		if err == nil || !strings.Contains(err.Error(), "解析响应失败") {
			t.Fatalf("err = %v, want 解析响应失败", err)
		}
	})
}
