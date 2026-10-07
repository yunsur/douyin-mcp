package main

// middleware_test.go covers authMiddleware, corsMiddleware and
// errorHandlingMiddleware. Each middleware runs on a throwaway gin router so
// the assertions observe exactly the middleware contract.

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// mwProbeRouter builds a minimal router whose /probe handler records whether it
// was reached, so middleware short-circuits are observable.
func mwProbeRouter(mw gin.HandlerFunc, reached *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mw)
	router.GET("/probe", func(c *gin.Context) {
		*reached = true
		c.String(http.StatusOK, "reached")
	})
	return router
}

func TestMiddlewareAuthEmptyTokenPasses(t *testing.T) {
	reached := false
	router := mwProbeRouter(authMiddleware(""), &reached)

	w := performRequest(router, http.MethodGet, "/probe", "", nil)

	if w.Code != http.StatusOK || w.Body.String() != "reached" {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
	}
	if !reached {
		t.Fatal("empty token should disable authentication")
	}
}

func TestMiddlewareAuthAcceptsBearerTokens(t *testing.T) {
	cases := []struct {
		name   string
		header string
	}{
		{"exact bearer", "Bearer secret-token"},
		{"scheme case-insensitive", "bearer secret-token"},
		{"extra spaces trimmed", "Bearer   secret-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			router := mwProbeRouter(authMiddleware("secret-token"), &reached)

			w := performRequest(router, http.MethodGet, "/probe", "",
				map[string]string{"Authorization": tc.header})

			if w.Code != http.StatusOK || w.Body.String() != "reached" {
				t.Fatalf("status = %d, body = %q", w.Code, w.Body.String())
			}
			if !reached {
				t.Fatal("valid Bearer token should reach the handler")
			}
			if got := w.Header().Get("WWW-Authenticate"); got != "" {
				t.Fatalf("unexpected WWW-Authenticate = %q", got)
			}
		})
	}
}

func TestMiddlewareAuthRejectsInvalidCredentials(t *testing.T) {
	// wrong-same-length and wrong-different-length both exercise the
	// constant-time comparison; malformed/missing headers short-circuit.
	cases := []struct {
		name   string
		header map[string]string
	}{
		{"missing header", nil},
		{"wrong token same length", map[string]string{"Authorization": "Bearer secret-tokeX"}},
		{"wrong token shorter", map[string]string{"Authorization": "Bearer sec"}},
		{"wrong scheme", map[string]string{"Authorization": "Basic secret-token"}},
		{"no scheme separator", map[string]string{"Authorization": "Bearersecret-token"}},
		{"empty credentials", map[string]string{"Authorization": "Bearer "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			router := mwProbeRouter(authMiddleware("secret-token"), &reached)

			w := performRequest(router, http.MethodGet, "/probe", "", tc.header)

			if code := decodeError(t, w, http.StatusUnauthorized); code != "UNAUTHORIZED" {
				t.Fatalf("error code = %q", code)
			}
			if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
			}
			if reached {
				t.Fatal("rejected request must not reach the handler")
			}
		})
	}
}

func TestMiddlewareCORSHeadersAndPreflight(t *testing.T) {
	reached := false
	router := gin.New()
	router.Use(corsMiddleware())
	router.GET("/probe", func(c *gin.Context) {
		reached = true
		c.String(http.StatusOK, "reached")
	})

	w := performRequest(router, http.MethodGet, "/probe", "", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !reached {
		t.Fatal("non-OPTIONS request should reach the handler")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, DELETE, OPTIONS" {
		t.Fatalf("Allow-Methods = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("Allow-Headers = %q", got)
	}

	reached = false
	preflight := performRequest(router, http.MethodOptions, "/probe", "", nil)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", preflight.Code)
	}
	if reached {
		t.Fatal("OPTIONS preflight must short-circuit before the handler")
	}
	if got := preflight.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("preflight Allow-Origin = %q", got)
	}
}

func TestMiddlewareRecoversPanicIntoErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(errorHandlingMiddleware())
	router.GET("/boom", func(c *gin.Context) {
		panic("kaboom")
	})

	w := performRequest(router, http.MethodGet, "/boom", "", nil)

	if code := decodeError(t, w, http.StatusInternalServerError); code != "INTERNAL_ERROR" {
		t.Fatalf("error code = %q", code)
	}
}
