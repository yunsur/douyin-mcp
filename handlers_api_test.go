package main

// handlers_api_test.go drives handlers_api.go end-to-end through the real
// router with a stub-backed DouyinService: one representative handler per
// family plus the shared error paths (bad body, upstream business error,
// non-JSON upstream body).

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/yunsur/douyin-mcp/douyin"
)

// handlerTestUpstream returns the first recorded request matching method+path
// fragment, failing the test with the recorded set otherwise.
func handlerTestUpstream(t *testing.T, st *mainStubTransport, method, substr string) mainStubRequest {
	t.Helper()
	for _, r := range st.requests() {
		if r.Method == method && strings.Contains(r.URL, substr) {
			return r
		}
	}
	var got []string
	for _, r := range st.requests() {
		got = append(got, r.Method+" "+r.URL)
	}
	t.Fatalf("no %s request containing %q; recorded: %v", method, substr, got)
	return mainStubRequest{}
}

// handlerTestDecodeArrayData asserts the success envelope for handlers whose
// data payload is a JSON array (decodeSuccess only handles objects).
func handlerTestDecodeArrayData(t *testing.T, w *httptest.ResponseRecorder) []any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var env struct {
		Success bool  `json:"success"`
		Data    []any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body %s)", err, w.Body.String())
	}
	if !env.Success {
		t.Fatalf("success = false, body = %s", w.Body.String())
	}
	return env.Data
}

func TestHandlerSessionLoginStatus(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{"user_uid": 4242})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodGet, "/api/v1/login/status", "", nil)

	data := decodeSuccess(t, w)
	if data["logged_in"] != true {
		t.Fatalf("logged_in = %v", data["logged_in"])
	}
	if uid, ok := data["uid"].(float64); !ok || uid != 4242 {
		t.Fatalf("uid = %v", data["uid"])
	}
	handlerTestUpstream(t, st, http.MethodGet, "/aweme/v1/web/query/user/")
}

func TestHandlerUserInfoRequestAndEnvelope(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{
		"status_code": 0,
		"user":        map[string]any{"nickname": "alice"},
	})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/user/info",
		`{"user":"https://www.douyin.com/user/SEC123"}`, nil)

	data := decodeSuccess(t, w)
	user, _ := data["user"].(map[string]any)
	if user["nickname"] != "alice" {
		t.Fatalf("user = %v", data["user"])
	}
	req := handlerTestUpstream(t, st, http.MethodGet, "/aweme/v1/web/user/profile/other/")
	if !strings.Contains(req.URL, "sec_user_id=SEC123") {
		t.Fatalf("upstream URL missing sec_user_id: %s", req.URL)
	}
}

func TestHandlerSearchVideosQueryPassthrough(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{"status_code": 0, "data": []any{}})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/search/videos",
		`{"keyword":"cat video"}`, nil)

	data := decodeSuccess(t, w)
	if _, ok := data["status_code"]; !ok {
		t.Fatalf("data = %v", data)
	}
	req := handlerTestUpstream(t, st, http.MethodGet, "/aweme/v1/web/general/search/single/")
	if !strings.Contains(req.URL, "keyword=cat") {
		t.Fatalf("upstream URL missing keyword: %s", req.URL)
	}
}

func TestHandlerSearchVideosMissingKeyword(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	// GET reads the keyword from the query string; POST would be rejected by
	// the binding:"required" tag first.
	w := performRequest(router, http.MethodGet, "/api/v1/search/videos", "", nil)

	if code := decodeError(t, w, http.StatusBadRequest); code != "MISSING_KEYWORD" {
		t.Fatalf("error code = %q", code)
	}
}

func TestHandlerNoticeCountUpstreamAndBusinessError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc, st := newTestServiceJSON(t, map[string]any{"status_code": 0, "comment": 3})
		router := newTestRouter(t, svc, "")

		w := performRequest(router, http.MethodGet, "/api/v1/notices/count", "", nil)

		data := decodeSuccess(t, w)
		if n, ok := data["comment"].(float64); !ok || n != 3 {
			t.Fatalf("comment = %v", data["comment"])
		}
		handlerTestUpstream(t, st, http.MethodGet, "/aweme/v1/web/notice/count/")
	})

	t.Run("non-zero status_code maps to mapped error", func(t *testing.T) {
		svc, st := newTestServiceJSON(t, map[string]any{"status_code": 20003, "status_msg": "need login"})
		router := newTestRouter(t, svc, "")

		w := performRequest(router, http.MethodGet, "/api/v1/notices/count", "", nil)

		if code := decodeError(t, w, http.StatusInternalServerError); code != "NOTICE_COUNT_FAILED" {
			t.Fatalf("error code = %q", code)
		}
		if !strings.Contains(w.Body.String(), "status_code=20003") {
			t.Fatalf("error details should surface upstream status_code: %s", w.Body.String())
		}
		handlerTestUpstream(t, st, http.MethodGet, "/aweme/v1/web/notice/count/")
	})
}

func TestHandlerDiggVideoSuccess(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{"status_code": 0})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/video/digg",
		`{"aweme_id":"7111111111111111111"}`, nil)

	data := decodeSuccess(t, w)
	if data["success"] != true {
		t.Fatalf("success = %v (body %s)", data["success"], w.Body.String())
	}
	req := handlerTestUpstream(t, st, http.MethodPost, "/aweme/v1/web/commit/item/digg/")
	if !strings.Contains(string(req.Body), "aweme_id=7111111111111111111") {
		t.Fatalf("digg body missing aweme_id: %s", string(req.Body))
	}
}

func TestHandlerLiveLikeRequest(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{"status_code": 0})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/live/like", `{"room_id":"987654"}`, nil)

	data := decodeSuccess(t, w)
	if _, ok := data["status_code"]; !ok {
		t.Fatalf("data = %v", data)
	}
	req := handlerTestUpstream(t, st, http.MethodPost, "/webcast/room/like/")
	if !strings.Contains(req.URL, "room_id=987654") {
		t.Fatalf("live like URL missing room_id: %s", req.URL)
	}
}

func TestHandlerIMUserInfoArrayEnvelope(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{
		"data": []any{map[string]any{"nickname": "bob", "sec_uid": "SECBOB"}},
	})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/im/user_info",
		`{"sec_uids":["SECBOB"]}`, nil)

	arr := handlerTestDecodeArrayData(t, w)
	if len(arr) != 1 {
		t.Fatalf("data length = %d (%v)", len(arr), arr)
	}
	first, _ := arr[0].(map[string]any)
	if first["nickname"] != "bob" {
		t.Fatalf("first entry = %v", first)
	}
	req := handlerTestUpstream(t, st, http.MethodPost, "/aweme/v1/web/im/user/info/")
	if !strings.Contains(string(req.Body), "sec_user_ids") {
		t.Fatalf("im user_info body = %s", string(req.Body))
	}
}

// publishStub answers the multi-step creator publish chain (upload auth →
// imagex apply → byte upload → commit → create_v2) so PublishContent succeeds
// offline. Every branch is keyed by method+host/path.
func publishStub(req mainStubRequest) (*douyin.Response, error) {
	u := req.URL
	switch {
	case req.Method == http.MethodHead && strings.Contains(u, "creator.douyin.com"):
		return &douyin.Response{
			StatusCode: http.StatusOK,
			Header:     fhttp.Header{"X-Ware-Csrf-Token": {"0,csrf-test-token,2,3,4"}},
			Body:       []byte("{}"),
		}, nil
	case strings.Contains(u, "/web/api/media/upload/auth/v5/"):
		return mainJSONResponse(map[string]any{
			"status_code": 0,
			"auth":        `{"AccessKeyID":"AK","SecretAccessKey":"SK","SessionToken":"ST"}`,
		}), nil
	case strings.Contains(u, "imagex.bytedanceapi.com") && req.Method == http.MethodGet:
		return mainJSONResponse(map[string]any{
			"Result": map[string]any{
				"UploadAddress": map[string]any{
					"StoreInfos":  []any{map[string]any{"StoreUri": "store-uri", "Auth": "upload-ticket", "UploadID": "upload-id"}},
					"UploadHosts": []any{"upload.example.com"},
					"SessionKey":  "session-key",
				},
			},
		}), nil
	case strings.Contains(u, "upload.example.com"):
		return mainJSONResponse(map[string]any{"code": 2000}), nil
	case strings.Contains(u, "imagex.bytedanceapi.com") && req.Method == http.MethodPost:
		return mainJSONResponse(map[string]any{
			"Result": map[string]any{
				"PluginResult": []any{map[string]any{
					"ImageUri": "store-uri", "ImageWidth": 100, "ImageHeight": 200,
					"ImageFormat": "png", "ImageSize": 10,
				}},
			},
		}), nil
	case strings.Contains(u, "/web/api/media/aweme/create_v2/"):
		return mainJSONResponse(map[string]any{"status_code": 0, "item_id": "7300000000000000000"}), nil
	default:
		return mainJSONResponse(map[string]any{"status_code": 0}), nil
	}
}

func TestHandlerPublishImageEndToEnd(t *testing.T) {
	svc, st := newTestService(t, publishStub)
	// Give the session publish security materials so requirePublishSecurity and
	// the bd-ticket-guard header construction succeed.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	prv := hex.EncodeToString(key.D.FillBytes(make([]byte, 32)))
	secure, err := douyin.NewClient("sessionid=abc; UIFID=uif", douyin.Options{
		Transport:  st,
		Ticket:     "ticket-abc",
		TsSign:     "ts.2.testsign",
		PrivateKey: prv,
		DtraitBlob: "dtrait-blob",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	svc.client = secure

	router := newTestRouter(t, svc, "")
	w := performRequest(router, http.MethodPost, "/api/v1/publish",
		`{"title":"hello","images":["https://cdn.example.com/a.png"]}`, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	data := decodeSuccess(t, w)
	if data["item_id"] != "7300000000000000000" {
		t.Fatalf("item_id = %v (body %s)", data["item_id"], w.Body.String())
	}
	handlerTestUpstream(t, st, http.MethodGet, "/web/api/media/upload/auth/v5/")
	handlerTestUpstream(t, st, http.MethodPost, "upload.example.com")
	createReq := handlerTestUpstream(t, st, http.MethodPost, "/web/api/media/aweme/create_v2/")
	if !strings.Contains(string(createReq.Body), "creation_id") {
		t.Fatalf("create_v2 body = %s", string(createReq.Body))
	}
}

func TestHandlerMalformedJSONBody(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	t.Run("syntax error", func(t *testing.T) {
		w := performRequest(router, http.MethodPost, "/api/v1/user/info", `{"user":`, nil)
		if code := decodeError(t, w, http.StatusBadRequest); code != "INVALID_REQUEST" {
			t.Fatalf("error code = %q", code)
		}
	})
	t.Run("missing required field", func(t *testing.T) {
		w := performRequest(router, http.MethodPost, "/api/v1/user/info", `{}`, nil)
		if code := decodeError(t, w, http.StatusBadRequest); code != "INVALID_REQUEST" {
			t.Fatalf("error code = %q", code)
		}
	})
}

func TestHandlerUpstreamNonJSONReturns500(t *testing.T) {
	svc, _ := newTestService(t, func(mainStubRequest) (*douyin.Response, error) {
		return &douyin.Response{StatusCode: http.StatusOK, Body: []byte("<html>not json</html>")}, nil
	})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/user/info",
		`{"user":"https://www.douyin.com/user/SEC123"}`, nil)

	if code := decodeError(t, w, http.StatusInternalServerError); code != "GET_USER_INFO_FAILED" {
		t.Fatalf("error code = %q (body %s)", code, w.Body.String())
	}
}

func TestJSONTextStringPassthrough(t *testing.T) {
	const raw = "already text, not JSON"
	if got := jsonText(raw); got != raw {
		t.Fatalf("jsonText(%q) = %q, want passthrough", raw, got)
	}
}

func TestJSONTextPrettyPrintsObjects(t *testing.T) {
	type payload struct {
		A int      `json:"a"`
		B []string `json:"b"`
	}
	got := jsonText(payload{A: 1, B: []string{"x", "y"}})
	want := "{\n  \"a\": 1,\n  \"b\": [\n    \"x\",\n    \"y\"\n  ]\n}"
	if got != want {
		t.Fatalf("jsonText = %q, want %q", got, want)
	}
}

func TestJSONTextUnmarshalableFallsBackToEmptyObject(t *testing.T) {
	got := jsonText(make(chan int))
	if got != "{}" {
		t.Fatalf("jsonText(chan) = %q, want {}", got)
	}
}

// An unsupported attachment kind is caller error: it must be a 400 and must not
// reach the upstream (no conversation may be created for a typo).
func TestHandlerSendDMMediaRejectsUnknownKind(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/im/send_media",
		`{"to_user_id":123456,"kind":"sticker","path":"/tmp/a.jpg"}`, nil)

	if code := decodeError(t, w, http.StatusBadRequest); code != "INVALID_REQUEST" {
		t.Fatalf("error code = %q, want INVALID_REQUEST", code)
	}
	var env struct {
		Details string `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if !strings.Contains(env.Details, "不支持的私信媒体类型") {
		t.Fatalf("details = %q", env.Details)
	}
	if got := len(st.requests()); got != 0 {
		t.Fatalf("rejected kind issued %d upstream requests", got)
	}
}

// Same for a blank room id on start_live_listen: the binding:"required" tag only
// rejects an empty string, so whitespace must be caught by the service — and it
// must answer 400, not blame the upstream with a 500.
func TestHandlerStartLiveListenRejectsBlankRoomID(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	w := performRequest(router, http.MethodPost, "/api/v1/live/listen/start",
		`{"web_rid":"   "}`, nil)

	if code := decodeError(t, w, http.StatusBadRequest); code != "INVALID_REQUEST" {
		t.Fatalf("error code = %q, want INVALID_REQUEST", code)
	}
	if got := len(st.requests()); got != 0 {
		t.Fatalf("blank room id issued %d upstream requests", got)
	}

	svc.liveMu.Lock()
	listener := svc.live
	svc.liveMu.Unlock()
	if listener != nil {
		t.Fatal("blank room id left a listener installed")
	}
}
