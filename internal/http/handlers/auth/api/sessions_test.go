package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/testutil"
)

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

	return session.New(session.SessionConfig{
		Store: tdb.Store,
		Tx:    tx.New(tdb.Store),
		AccessTokens: token.NewAccessTokenService(
			keySet,
			"authara-test",
			time.Minute,
		),
		SessionTTL:      time.Hour,
		RefreshTokenTTL: time.Hour,
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
