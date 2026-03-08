package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexlup06-authgate/authgate/internal/http/kit/csrf"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

func TestStableCookieNames_AccessAndRefresh(t *testing.T) {
	rr := httptest.NewRecorder()

	session.SetAccessToken(rr, "access-token-value", 3600)
	session.SetRefreshToken(rr, "refresh-token-value", 7200)

	resp := rr.Result()
	cookies := resp.Cookies()

	var foundAccess bool
	var foundRefresh bool

	for _, c := range cookies {
		switch c.Name {
		case "authgate_access":
			foundAccess = true
		case "authgate_refresh":
			foundRefresh = true
		}
	}

	if !foundAccess {
		t.Fatal("expected cookie authgate_access to be set")
	}
	if !foundRefresh {
		t.Fatal("expected cookie authgate_refresh to be set")
	}
}

func TestStableCookieNames_ClearSessionCookies(t *testing.T) {
	rr := httptest.NewRecorder()

	session.ClearSessionCookies(rr)

	resp := rr.Result()
	cookies := resp.Cookies()

	var foundAccess bool
	var foundRefresh bool

	for _, c := range cookies {
		switch c.Name {
		case "authgate_access":
			foundAccess = true
		case "authgate_refresh":
			foundRefresh = true
		}
	}

	if !foundAccess {
		t.Fatal("expected cookie authgate_access to be cleared")
	}
	if !foundRefresh {
		t.Fatal("expected cookie authgate_refresh to be cleared")
	}
}

func TestStableCookieNames_CSRF(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)

	_, err := csrf.EnsureCookie(rr, req)
	if err != nil {
		t.Fatalf("EnsureCookie returned error: %v", err)
	}

	resp := rr.Result()
	cookies := resp.Cookies()

	var foundCSRF bool
	for _, c := range cookies {
		if c.Name == "authgate_csrf" {
			foundCSRF = true
			break
		}
	}

	if !foundCSRF {
		t.Fatal("expected cookie authgate_csrf to be set")
	}
}
