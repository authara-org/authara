package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	redir "github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/middleware"
)

var returnToPaths = []string{"/auth/login", "/auth/signup", "/auth/account"}

func newReturnToHandler() http.Handler {
	return middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context())
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))
}

func newReturnToHandlerWithDefault(defaultReturnTo string) http.Handler {
	return middleware.ReturnToWithDefault(defaultReturnTo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context())
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))
}

func TestRedirectContract_ReturnTo_AppliesToPaths(t *testing.T) {
	handler := newReturnToHandler()
	for _, path := range returnToPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?"+redirect.ReturnToQueryParam+"=/private", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/private" {
				t.Fatalf("expected Location=/private, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_InvalidValueFallsBackToDefault(t *testing.T) {
	handler := newReturnToHandler()
	for _, path := range returnToPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?return_to=//evil.com", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/" {
				t.Fatalf("expected Location=/, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_NoValueFallsBackToDefault(t *testing.T) {
	handler := newReturnToHandler()
	for _, path := range returnToPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/" {
				t.Fatalf("expected Location=/, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_NoValueUsesConfiguredDefault(t *testing.T) {
	handler := newReturnToHandlerWithDefault("/dashboard")
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
	}

	if got := rr.Header().Get("Location"); got != "/dashboard" {
		t.Fatalf("expected Location=/dashboard, got %q", got)
	}
}

func TestRedirectContract_ReturnTo_HTMXUsesHXRedirect(t *testing.T) {
	handler := newReturnToHandler()
	for _, path := range returnToPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?"+redirect.ReturnToQueryParam+"=/private", nil)
			req.Header.Set("HX-Request", "true")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
			}

			if got := rr.Header().Get("HX-Redirect"); got != "/private" {
				t.Fatalf("expected HX-Redirect=/private, got %q", got)
			}

			if got := rr.Header().Get("Location"); got != "" {
				t.Fatalf("expected no Location header for HTMX redirect, got %q", got)
			}
		})
	}
}
