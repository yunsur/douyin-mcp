package main

// Shared hermetic test harness for the main package.
//
// mainStubTransport implements douyin.Transport so handler, route and service
// tests can run end-to-end without network access: requests are recorded and
// responses come from the test. Keep this file free of test cases; it is
// reused by every *_test.go file in the package.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yunsur/douyin-mcp/configs"
	"github.com/yunsur/douyin-mcp/douyin"
)

// mainStubRequest is one recorded outgoing request.
type mainStubRequest struct {
	Method  string
	URL     string
	Headers douyin.Headers
	Cookie  string
	Body    []byte
}

// mainStubTransport records requests and answers with the test handler.
type mainStubTransport struct {
	mu     sync.Mutex
	reqs   []mainStubRequest
	handle func(mainStubRequest) (*douyin.Response, error)
}

// Do implements douyin.Transport.
func (s *mainStubTransport) Do(ctx context.Context, method, rawURL string, headers douyin.Headers, cookieHeader string, body []byte) (*douyin.Response, error) {
	rec := mainStubRequest{Method: method, URL: rawURL, Headers: headers, Cookie: cookieHeader, Body: body}

	s.mu.Lock()
	s.reqs = append(s.reqs, rec)
	handle := s.handle
	s.mu.Unlock()

	if handle == nil {
		return mainJSONResponse(map[string]any{}), nil
	}
	return handle(rec)
}

// Get implements douyin.Transport.
func (s *mainStubTransport) Get(ctx context.Context, rawURL string, headers douyin.Headers, cookieHeader string) (*douyin.Response, error) {
	return s.Do(ctx, http.MethodGet, rawURL, headers, cookieHeader, nil)
}

// PostJSON implements douyin.Transport.
func (s *mainStubTransport) PostJSON(ctx context.Context, rawURL string, headers douyin.Headers, cookieHeader, contentType string, body []byte) (*douyin.Response, error) {
	if contentType != "" {
		headers.Set("content-type", contentType)
	}
	return s.Do(ctx, http.MethodPost, rawURL, headers, cookieHeader, body)
}

// requests returns a copy of the recorded requests.
func (s *mainStubTransport) requests() []mainStubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mainStubRequest(nil), s.reqs...)
}

// last returns the most recent request, failing the test when none was made.
func (s *mainStubTransport) last(t *testing.T) mainStubRequest {
	t.Helper()
	reqs := s.requests()
	if len(reqs) == 0 {
		t.Fatal("no request was issued")
	}
	return reqs[len(reqs)-1]
}

// mainJSONResponse builds a 200 response carrying v as JSON.
func mainJSONResponse(v any) *douyin.Response {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return &douyin.Response{StatusCode: http.StatusOK, Body: body}
}

// newTestService builds a DouyinService backed by a stub transport.
func newTestService(t *testing.T, handle func(mainStubRequest) (*douyin.Response, error)) (*DouyinService, *mainStubTransport) {
	t.Helper()
	st := &mainStubTransport{handle: handle}
	client, err := douyin.NewClient("sessionid=abc; UIFID=uif", douyin.Options{Transport: st})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	dir := t.TempDir()
	return &DouyinService{
		cfg:           &configs.Config{Port: ":18080"},
		cookiePath:    filepath.Join(dir, "cookies.txt"),
		client:        client,
		convCache:     map[string]douyin.IMConversation{},
		convCachePath: filepath.Join(dir, "conversations_cache.json"),
	}, st
}

// newTestServiceJSON builds a service that always answers with the same JSON body.
func newTestServiceJSON(t *testing.T, v any) (*DouyinService, *mainStubTransport) {
	t.Helper()
	return newTestService(t, func(mainStubRequest) (*douyin.Response, error) { return mainJSONResponse(v), nil })
}

// newTestRouter builds the full gin router around a stub-backed service.
func newTestRouter(t *testing.T, svc *DouyinService, token string) *gin.Engine {
	t.Helper()
	return setupRoutes(NewAppServer(svc, token))
}

// performRequest runs one HTTP request through the router.
func performRequest(router http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// decodeSuccess asserts the standard success envelope and returns the data object.
func decodeSuccess(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var env struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
		Message string         `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body %s)", err, w.Body.String())
	}
	if !env.Success {
		t.Fatalf("success = false, body = %s", w.Body.String())
	}
	return env.Data
}

// decodeError asserts the standard error envelope and returns the error code.
func decodeError(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) string {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, wantStatus, w.Body.String())
	}
	var env struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v (body %s)", err, w.Body.String())
	}
	if env.Error == "" {
		t.Fatalf("error message is empty, body = %s", w.Body.String())
	}
	return env.Code
}
