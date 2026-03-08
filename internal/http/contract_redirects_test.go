package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	redir "github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/middleware"
)

func TestRedirectContract_ReturnTo_RegularBrowserRequest(t *testing.T) {
	handler := middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context(), "/")
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))

	req := httptest.NewRequest(http.MethodGet, "/auth/login?return_to=/private", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	if got := rr.Header().Get("Location"); got != "/private" {
		t.Fatalf("expected Location=/private, got %q", got)
	}

	if got := rr.Header().Get("HX-Redirect"); got != "/private" {
		t.Fatalf("expected HX-Redirect=/private, got %q", got)
	}
}

func TestRedirectContract_ReturnTo_InvalidValueFallsBackToDefault(t *testing.T) {
	handler := middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context(), "/")
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))

	req := httptest.NewRequest(http.MethodGet, "/auth/login?return_to=//evil.com", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("expected Location=/, got %q", got)
	}

	if got := rr.Header().Get("HX-Redirect"); got != "/" {
		t.Fatalf("expected HX-Redirect=/, got %q", got)
	}
}

func TestRedirectContract_HTMXUsesHXRedirect(t *testing.T) {
	handler := middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context(), "/")
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))

	req := httptest.NewRequest(http.MethodGet, "/auth/login?return_to=/private", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// HTMX redirect helper intentionally returns 200 + HX-Redirect
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if got := rr.Header().Get("HX-Redirect"); got != "/private" {
		t.Fatalf("expected HX-Redirect=/private, got %q", got)
	}

	// For HTMX redirects we do not expect a standard browser redirect location
	if got := rr.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header for HTMX redirect, got %q", got)
	}
}

func TestRedirectContract_NoReturnToFallsBackToDefault(t *testing.T) {
	handler := middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context(), "/")
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	if got := rr.Header().Get("Location"); got != "/" {
		t.Fatalf("expected Location=/, got %q", got)
	}

	if got := rr.Header().Get("HX-Redirect"); got != "/" {
		t.Fatalf("expected HX-Redirect=/, got %q", got)
	}
}
