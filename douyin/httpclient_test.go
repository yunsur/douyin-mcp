package douyin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression: a POST with a body must not carry a duplicate content-length.
// Setting it by hand (in addition to the transport's own) made every
// body-bearing request to Douyin fail with HTTP 400.
func TestHTTPClientPostBodyHasSingleContentLength(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Values("Content-Length")...)
		body, _ := readAll(r)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("proto-bytes")
	resp, err := c.Do(t.Context(), "POST", srv.URL, Headers{{Name: "content-type", Value: "application/x-protobuf"}}, "a=b", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || string(resp.Body) != string(body) {
		t.Fatalf("status=%d body=%q", resp.StatusCode, resp.Body)
	}
	if len(seen) != 1 {
		t.Fatalf("content-length sent %d times (%v), want exactly 1", len(seen), seen)
	}
	if strings.TrimSpace(seen[0]) != "11" {
		t.Fatalf("content-length=%q, want 11", seen[0])
	}
}

func readAll(r *http.Request) ([]byte, error) {
	buf := make([]byte, 0, 64)
	tmp := make([]byte, 64)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}
