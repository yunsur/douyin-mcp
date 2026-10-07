package main

// routes_test.go covers the wiring in setupRoutes: public vs protected
// endpoints, the MCP handler gate, 404s and gin's method-mismatch behavior.

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRoutesHealthIsPublic(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "configured-token")

	w := performRequest(router, http.MethodGet, "/health", "", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("health body = %s", w.Body.String())
	}
	if reqs := st.requests(); len(reqs) != 0 {
		t.Fatalf("health must not touch upstream, got %d requests", len(reqs))
	}
}

func TestRoutesAPIProtectedWhenTokenConfigured(t *testing.T) {
	svc, st := newTestServiceJSON(t, map[string]any{"user_uid": 99})
	router := newTestRouter(t, svc, "configured-token")

	t.Run("missing credentials", func(t *testing.T) {
		w := performRequest(router, http.MethodGet, "/api/v1/login/status", "", nil)
		if code := decodeError(t, w, http.StatusUnauthorized); code != "UNAUTHORIZED" {
			t.Fatalf("error code = %q", code)
		}
	})
	t.Run("wrong credentials", func(t *testing.T) {
		w := performRequest(router, http.MethodGet, "/api/v1/login/status", "",
			map[string]string{"Authorization": "Bearer nope"})
		if code := decodeError(t, w, http.StatusUnauthorized); code != "UNAUTHORIZED" {
			t.Fatalf("error code = %q", code)
		}
	})
	t.Run("valid credentials reach handler", func(t *testing.T) {
		w := performRequest(router, http.MethodGet, "/api/v1/login/status", "",
			map[string]string{"Authorization": "Bearer configured-token"})
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
		}
		if len(st.requests()) == 0 {
			t.Fatal("handler should have issued an upstream request")
		}
	})

	// With no token configured the same route is open.
	openSvc, _ := newTestServiceJSON(t, map[string]any{"user_uid": 99})
	openRouter := newTestRouter(t, openSvc, "")
	w := performRequest(openRouter, http.MethodGet, "/api/v1/login/status", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unauthenticated router status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRoutesMCPIsProtected(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "configured-token")

	denied := performRequest(router, http.MethodPost, "/mcp", `{}`, nil)
	if code := decodeError(t, denied, http.StatusUnauthorized); code != "UNAUTHORIZED" {
		t.Fatalf("error code = %q", code)
	}
	if got := denied.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}

	allowed := performRequest(router, http.MethodPost, "/mcp", `{}`,
		map[string]string{"Authorization": "Bearer configured-token"})
	if allowed.Code == http.StatusUnauthorized {
		t.Fatal("valid token must pass the auth gate on /mcp")
	}
}

func TestRoutesUnknownPathsReturnNotFound(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	for _, path := range []string{"/api/v1/definitely-not-a-route", "/nope"} {
		w := performRequest(router, http.MethodGet, path, "", nil)
		if code := decodeError(t, w, http.StatusNotFound); code != "NOT_FOUND" {
			t.Fatalf("GET %s error code = %q, want NOT_FOUND", path, code)
		}
	}
}

func TestRoutesMethodMismatch(t *testing.T) {
	svc, _ := newTestServiceJSON(t, map[string]any{})
	router := newTestRouter(t, svc, "")

	// /health is registered GET-only: a wrong method is a 405, not a 404 that
	// would hide the fact that the path exists.
	w := performRequest(router, http.MethodPost, "/health", "", nil)
	if code := decodeError(t, w, http.StatusMethodNotAllowed); code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("POST /health error code = %q, want METHOD_NOT_ALLOWED", code)
	}

	// /api/v1/video/digg is POST-only in the API group.
	apiW := performRequest(router, http.MethodGet, "/api/v1/video/digg", "", nil)
	if code := decodeError(t, apiW, http.StatusMethodNotAllowed); code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("GET /api/v1/video/digg error code = %q, want METHOD_NOT_ALLOWED", code)
	}
}
