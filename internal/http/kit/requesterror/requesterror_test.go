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
