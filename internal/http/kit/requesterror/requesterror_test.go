package requesterror

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func TestRenderFullPageErrorForRegularRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth/admin", nil)
	rr := httptest.NewRecorder()

	if err := Render(nil, rr, req, http.StatusForbidden, "No access."); err != nil {
		t.Fatalf("render error: %v", err)
	}

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Forbidden") || !strings.Contains(body, "No access.") {
		t.Fatalf("expected full error page, body=%s", body)
	}
}

func TestRenderToastForHTMXRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/admin/users/search", nil)
	req = req.WithContext(httpctx.WithHTMX(req.Context()))
	rr := httptest.NewRecorder()

	if err := Render(nil, rr, req, http.StatusInternalServerError, "Could not load user."); err != nil {
		t.Fatalf("render error: %v", err)
	}

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
	if got := rr.Header().Get(htmx.HTMXReswap); got != "none" {
		t.Fatalf("expected HX-Reswap none, got %q", got)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `hx-swap-oob="afterbegin:#toast-container"`) || !strings.Contains(body, "Could not load user.") {
		t.Fatalf("expected toast response, body=%s", body)
	}
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Fatalf("expected partial toast response, body=%s", body)
	}
}

func TestRenderRedirectsToConfiguredCustomPage(t *testing.T) {
	target := "https://app.example/errors/forbidden"
	handler := CustomPages(func(status int) string {
		if status == http.StatusForbidden {
			return target
		}
		return ""
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Render(nil, w, r, http.StatusForbidden, "No access."); err != nil {
			t.Fatalf("render error: %v", err)
		}
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/auth/account", nil))

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if location := rr.Header().Get("Location"); location != target {
		t.Fatalf("location = %q, want %q", location, target)
	}
}

func TestRenderKeepsHTMXErrorsInlineWhenCustomPageIsConfigured(t *testing.T) {
	handler := CustomPages(func(int) string { return "https://app.example/errors/server" })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Render(nil, w, r, http.StatusInternalServerError, "Could not load user."); err != nil {
			t.Fatalf("render error: %v", err)
		}
	}))

	req := httptest.NewRequest(http.MethodPost, "/auth/account", nil)
	req.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError || rr.Header().Get("Location") != "" {
		t.Fatalf("status/location = %d/%q", rr.Code, rr.Header().Get("Location"))
	}
}
