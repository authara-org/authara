package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/config"
)

func TestRequireAllowlistEnabledAllowsRequestWhenEnabled(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist", nil)

	RequireAllowlistEnabled(true)(next).ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
}

func TestRequireAllowlistEnabledReadsCurrentPolicy(t *testing.T) {
	enabled := false
	policy := config.AllowlistPolicyReaderFunc(func() config.AllowlistPolicy {
		return config.AllowlistPolicy{AllowlistEnabled: enabled}
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequireAllowlistEnabledWithPolicy(policy)(next)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disabled status = %d", rr.Code)
	}

	enabled = true
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("enabled status = %d", rr.Code)
	}
}

func TestRequireAllowlistEnabledReturnsNotFoundWhenDisabled(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist", nil)

	RequireAllowlistEnabled(false)(next).ServeHTTP(rr, req)

	if called {
		t.Fatal("expected next handler not to be called")
	}
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
