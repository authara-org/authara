package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersSetsSafeDefaults(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	headers := rr.Result().Header

	if got := headers.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", got)
	}
	if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options nosniff, got %q", got)
	}
	if got := headers.Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("expected Referrer-Policy same-origin, got %q", got)
	}

	csp := headers.Get("Content-Security-Policy")
	requireCSPContains(t, csp,
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"font-src 'self'",
		"img-src 'self' data:",
		"connect-src 'self'",
		"frame-src 'none'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
		"style-src 'self' 'unsafe-inline'",
	)

	if strings.Contains(csp, "accounts.google.com") {
		t.Fatalf("did not expect Google CSP sources without Google OAuth, got %q", csp)
	}
}

func TestSecurityHeadersAllowsGoogleOAuthSourcesWhenEnabled(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{AllowGoogleOAuth: true})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	csp := rr.Result().Header.Get("Content-Security-Policy")
	requireCSPContains(t, csp,
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://accounts.google.com",
		"connect-src 'self' https://accounts.google.com",
		"frame-src https://accounts.google.com",
		"img-src 'self' data: https://www.gstatic.com https://ssl.gstatic.com",
	)
}

func requireCSPContains(t *testing.T, csp string, expected ...string) {
	t.Helper()

	if csp == "" {
		t.Fatal("expected Content-Security-Policy header to be set")
	}

	for _, directive := range expected {
		if !strings.Contains(csp, directive) {
			t.Fatalf("expected CSP to contain %q, got %q", directive, csp)
		}
	}
}
