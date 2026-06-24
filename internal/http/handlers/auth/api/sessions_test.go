package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/testutil"
)

func TestRefreshPostSetsCookiesOnly(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().Add(-time.Minute)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-refresh-tokens@example.com",
			Username: "api-refresh-tokens",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		sessionService := newAPIHandlerTestSessionService(t, tdb)
		_, refreshToken, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, "test-agent", now)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		h := &APIHandler{
			Session:    sessionService,
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/sessions/refresh", nil).WithContext(ctx)
		addRefreshCookie(req, refreshToken)
		rr := httptest.NewRecorder()

		h.RefreshPost(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if !hasCookie(rr.Result().Cookies(), "authara_access") || !hasCookie(rr.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected refresh to set session cookies")
		}
		if rr.Body.Len() != 0 {
			t.Fatalf("expected empty response body, got %q", rr.Body.String())
		}
	})
}

func TestTokenRefreshPostReturnsTokensFromBody(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().Add(-time.Minute)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-token-refresh@example.com",
			Username: "api-token-refresh",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		sessionService := newAPIHandlerTestSessionService(t, tdb)
		_, refreshToken, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, "test-agent", now)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		h := &APIHandler{
			Session:    sessionService,
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		body := `{"refresh_token":"` + refreshToken + `","audience":"app"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/tokens/refresh", strings.NewReader(body)).WithContext(ctx)
		rr := httptest.NewRecorder()

		h.TokenRefreshPost(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if hasCookie(rr.Result().Cookies(), "authara_access") || hasCookie(rr.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected token refresh not to set session cookies")
		}
		assertResponseTokens(t, rr.Body.Bytes())
	})
}

func TestRefreshPostDisabledUserReturnsUnauthorized(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-refresh-disabled@example.com",
			Username: "api-refresh-disabled",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		sessionService := newAPIHandlerTestSessionService(t, tdb)
		_, refreshToken, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, "test-agent", now)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		if err := tdb.Store.DisableUser(ctx, user.ID, now.Add(time.Minute)); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		h := &APIHandler{
			Session:    sessionService,
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/sessions/refresh", nil).WithContext(ctx)
		addRefreshCookie(req, refreshToken)
		rr := httptest.NewRecorder()

		h.RefreshPost(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
		}
		if !hasClearedRefreshCookie(rr.Result().Cookies()) {
			t.Fatal("expected refresh cookie to be cleared")
		}
	})
}

func newAPIHandlerTestSessionService(t *testing.T, tdb *testutil.TestDB) *session.Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}
	txManager := tx.New(tdb.Store)

	return session.New(session.SessionConfig{
		Store: tdb.Store,
		Tx:    txManager,
		AccessTokens: token.NewAccessTokenService(
			keySet,
			"authara-test",
			time.Minute,
		),
		SessionTTL:      time.Hour,
		RefreshTokenTTL: time.Hour,
		Organizations:   organization.New(organization.Config{Store: tdb.Store, Tx: txManager}),
	})
}

func addRefreshCookie(req *http.Request, refreshToken string) {
	rr := httptest.NewRecorder()
	session.SetRefreshToken(rr, refreshToken, 3600)
	for _, cookie := range rr.Result().Cookies() {
		req.AddCookie(cookie)
	}
}

func hasClearedRefreshCookie(cookies []*http.Cookie) bool {
	for _, cookie := range cookies {
		if cookie.Name == "authara_refresh" && cookie.MaxAge < 0 {
			return true
		}
	}
	return false
}
