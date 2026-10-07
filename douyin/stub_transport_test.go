package douyin

// Shared hermetic test harness for the douyin package.
//
// stubTransport implements Transport so tests can drive every Client method
// without touching the network: requests are recorded and responses are
// supplied by the test. Keep this file free of test cases; it is reused by all
// *_test.go files in the package.

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

// stubRequest is one recorded outgoing request.
type stubRequest struct {
	Method  string
	URL     string
	Headers Headers
	Cookie  string
	Body    []byte
}

// Header returns a recorded request header value.
func (r stubRequest) Header(name string) string {
	v, _ := r.Headers.Get(name)
	return v
}

// stubTransport records every request and answers with the test handler's
// response. A nil handler answers 200 with an empty JSON object.
type stubTransport struct {
	mu     sync.Mutex
	reqs   []stubRequest
	handle func(stubRequest) (*Response, error)
}

// Do implements Transport.
func (s *stubTransport) Do(ctx context.Context, method, rawURL string, headers Headers, cookieHeader string, body []byte) (*Response, error) {
	rec := stubRequest{Method: method, URL: rawURL, Headers: headers, Cookie: cookieHeader, Body: body}

	s.mu.Lock()
	s.reqs = append(s.reqs, rec)
	handle := s.handle
	s.mu.Unlock()

	if handle == nil {
		return jsonResponse(map[string]any{}), nil
	}
	return handle(rec)
}

// Get implements Transport.
func (s *stubTransport) Get(ctx context.Context, rawURL string, headers Headers, cookieHeader string) (*Response, error) {
	return s.Do(ctx, fhttp.MethodGet, rawURL, headers, cookieHeader, nil)
}

// PostJSON implements Transport, mirroring HTTPClient.PostJSON's content-type handling.
func (s *stubTransport) PostJSON(ctx context.Context, rawURL string, headers Headers, cookieHeader, contentType string, body []byte) (*Response, error) {
	if contentType != "" {
		headers.Set("content-type", contentType)
	}
	return s.Do(ctx, fhttp.MethodPost, rawURL, headers, cookieHeader, body)
}

// requests returns a copy of the recorded requests.
func (s *stubTransport) requests() []stubRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]stubRequest(nil), s.reqs...)
}

// last returns the most recent request, failing the test when none was made.
func (s *stubTransport) last(t *testing.T) stubRequest {
	t.Helper()
	reqs := s.requests()
	if len(reqs) == 0 {
		t.Fatal("no request was issued")
	}
	return reqs[len(reqs)-1]
}

// jsonResponse builds a 200 response with v marshalled as JSON.
func jsonResponse(v any) *Response {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return &Response{StatusCode: http.StatusOK, Body: body}
}

// statusResponse builds a response with the given HTTP status and body.
func statusResponse(status int, body string) *Response {
	return &Response{StatusCode: status, Body: []byte(body)}
}

// ck builds a response cookie for absorbCookies tests.
func ck(name, value string) *fhttp.Cookie { return &fhttp.Cookie{Name: name, Value: value} }

// newStubClient returns a Client whose transport is controlled by handle.
func newStubClient(t *testing.T, handle func(stubRequest) (*Response, error)) (*Client, *stubTransport) {
	t.Helper()
	st := &stubTransport{handle: handle}
	c, err := NewClient("sessionid=abc; UIFID=uif", Options{Transport: st})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, st
}

// newStubJSON returns a Client that always answers with the same JSON body.
func newStubJSON(t *testing.T, v any) (*Client, *stubTransport) {
	t.Helper()
	return newStubClient(t, func(stubRequest) (*Response, error) { return jsonResponse(v), nil })
}
