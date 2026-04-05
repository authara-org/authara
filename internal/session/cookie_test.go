package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetAccessToken(t *testing.T) {
	rec := httptest.NewRecorder()

	SetAccessToken(rec, "access-token-value", 3600)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != accessCookieName {
		t.Fatalf("expected cookie name %q, got %q", accessCookieName, c.Name)
	}
	if c.Value != "access-token-value" {
		t.Fatalf("expected cookie value %q, got %q", "access-token-value", c.Value)
	}
	if c.Path != cookiePath {
		t.Fatalf("expected cookie path %q, got %q", cookiePath, c.Path)
	}
	if !c.HttpOnly {
		t.Fatal("expected HttpOnly to be true")
	}
	if c.Secure != secureCookies {
		t.Fatalf("expected Secure=%v, got %v", secureCookies, c.Secure)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=%v, got %v", http.SameSiteLaxMode, c.SameSite)
	}
	if c.MaxAge != 3600 {
		t.Fatalf("expected MaxAge=3600, got %d", c.MaxAge)
	}
	if c.Expires.IsZero() {
		t.Fatal("expected Expires to be set")
	}
	if time.Until(c.Expires) <= 0 {
		t.Fatal("expected Expires to be in the future")
	}
}

func TestSetRefreshToken(t *testing.T) {
	rec := httptest.NewRecorder()

	SetRefreshToken(rec, "refresh-token-value", 7200)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != refreshCookieName {
		t.Fatalf("expected cookie name %q, got %q", refreshCookieName, c.Name)
	}
	if c.Value != "refresh-token-value" {
		t.Fatalf("expected cookie value %q, got %q", "refresh-token-value", c.Value)
	}
	if c.Path != cookiePath {
		t.Fatalf("expected cookie path %q, got %q", cookiePath, c.Path)
	}
	if !c.HttpOnly {
		t.Fatal("expected HttpOnly to be true")
	}
	if c.Secure != secureCookies {
		t.Fatalf("expected Secure=%v, got %v", secureCookies, c.Secure)
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected SameSite=%v, got %v", http.SameSiteLaxMode, c.SameSite)
	}
	if c.MaxAge != 7200 {
		t.Fatalf("expected MaxAge=7200, got %d", c.MaxAge)
	}
	if c.Expires.IsZero() {
		t.Fatal("expected Expires to be set")
	}
	if time.Until(c.Expires) <= 0 {
		t.Fatal("expected Expires to be in the future")
	}
}

func TestClearSessionCookies(t *testing.T) {
	rec := httptest.NewRecorder()

	ClearSessionCookies(rec)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	found := map[string]*http.Cookie{}
	for _, c := range cookies {
		found[c.Name] = c
	}

	access, ok := found[accessCookieName]
	if !ok {
		t.Fatalf("expected %q cookie to be set", accessCookieName)
	}
	refresh, ok := found[refreshCookieName]
	if !ok {
		t.Fatalf("expected %q cookie to be set", refreshCookieName)
	}

	for _, c := range []*http.Cookie{access, refresh} {
		if c.Value != "" {
			t.Fatalf("expected cleared cookie value to be empty, got %q", c.Value)
		}
		if c.Path != cookiePath {
			t.Fatalf("expected cookie path %q, got %q", cookiePath, c.Path)
		}
		if !c.HttpOnly {
			t.Fatal("expected HttpOnly to be true")
		}
		if c.Secure != secureCookies {
			t.Fatalf("expected Secure=%v, got %v", secureCookies, c.Secure)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Fatalf("expected SameSite=%v, got %v", http.SameSiteLaxMode, c.SameSite)
		}
		if c.MaxAge != -1 {
			t.Fatalf("expected MaxAge=-1, got %d", c.MaxAge)
		}
		if !c.Expires.Equal(time.Unix(0, 0)) {
			t.Fatalf("expected Expires=%v, got %v", time.Unix(0, 0), c.Expires)
		}
	}
}

func TestReadAccessToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  accessCookieName,
		Value: "access-token-value",
	})

	got, ok := ReadAccessToken(req)
	if !ok {
		t.Fatal("expected access token to be found")
	}
	if got != "access-token-value" {
		t.Fatalf("expected token %q, got %q", "access-token-value", got)
	}
}

func TestReadAccessToken_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got, ok := ReadAccessToken(req)
	if ok {
		t.Fatal("expected access token to be missing")
	}
	if got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}

func TestReadRefreshToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  refreshCookieName,
		Value: "refresh-token-value",
	})

	got, ok := ReadRefreshToken(req)
	if !ok {
		t.Fatal("expected refresh token to be found")
	}
	if got != "refresh-token-value" {
		t.Fatalf("expected token %q, got %q", "refresh-token-value", got)
	}
}

func TestReadRefreshToken_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got, ok := ReadRefreshToken(req)
	if ok {
		t.Fatal("expected refresh token to be missing")
	}
	if got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}
