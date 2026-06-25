package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteOAuthRedirectUsesFetchHeader(t *testing.T) {
	rr := httptest.NewRecorder()

	writeOAuthRedirect(rr, "/")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Header().Get("X-Authara-Redirect"); got != "/" {
		t.Fatalf("expected redirect header /, got %q", got)
	}
	if got := rr.Header().Get("Location"); got != "" {
		t.Fatalf("expected no Location header, got %q", got)
	}
}

func TestIsOAuthCallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/auth/oauth/google/callback", nil)
	if !isOAuthCallback(req) {
		t.Fatalf("expected google callback request to match")
	}

	req = httptest.NewRequest(http.MethodPost, "/auth/invitations/login", nil)
	if isOAuthCallback(req) {
		t.Fatalf("expected non-callback request not to match")
	}
}
