package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
)

func TestRequireAccessAuthWithRefreshSwitchesBetweenAppAndOperatorAudiences(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
		sessionService := newAudienceSwitchSessionService(t, tdb)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "operator-switch@example.com",
			Username: "operator-switch",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		if err := tdb.Store.AddUserPlatformRoleByName(ctx, user.ID, roles.DBOperatorRoleName); err != nil {
			t.Fatalf("AddUserPlatformRoleByName failed: %v", err)
		}

		appAccess, refreshToken, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, "test-agent", now, "")
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		operatorHandler := RequireAccessAuthWithRefresh(
			sessionService,
			token.AudienceOperator,
			10*time.Minute,
			time.Hour,
			func() time.Time { return now.Add(time.Minute) },
		)(RequireOperator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestRoles, ok := httpctx.Roles(r.Context())
			if !ok || !requestRoles.IsOperator() {
				t.Fatal("expected switched request to contain the operator role")
			}
			w.WriteHeader(http.StatusNoContent)
		})))

		operatorRequest := httptest.NewRequest(http.MethodGet, "/auth/operator", nil).WithContext(ctx)
		addSessionCookies(operatorRequest, appAccess, refreshToken)
		operatorResponse := httptest.NewRecorder()
		operatorHandler.ServeHTTP(operatorResponse, operatorRequest)

		if operatorResponse.Code != http.StatusNoContent {
			t.Fatalf("operator request status = %d, want %d", operatorResponse.Code, http.StatusNoContent)
		}
		operatorAccess, operatorRefresh := switchedSessionCookies(t, operatorResponse)
		if _, err := sessionService.ValidateAccessToken(ctx, operatorAccess, token.AudienceOperator, now.Add(time.Minute)); err != nil {
			t.Fatalf("operator access token is not valid for operator audience: %v", err)
		}

		appHandler := RequireAccessAuthWithRefresh(
			sessionService,
			token.AudienceApp,
			10*time.Minute,
			time.Hour,
			func() time.Time { return now.Add(2 * time.Minute) },
		)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		appRequest := httptest.NewRequest(http.MethodGet, "/auth/account", nil).WithContext(ctx)
		addSessionCookies(appRequest, operatorAccess, operatorRefresh)
		appResponse := httptest.NewRecorder()
		appHandler.ServeHTTP(appResponse, appRequest)

		if appResponse.Code != http.StatusNoContent {
			t.Fatalf("app request status = %d, want %d", appResponse.Code, http.StatusNoContent)
		}
		switchedAppAccess, _ := switchedSessionCookies(t, appResponse)
		if _, err := sessionService.ValidateAccessToken(ctx, switchedAppAccess, token.AudienceApp, now.Add(2*time.Minute)); err != nil {
			t.Fatalf("switched access token is not valid for app audience: %v", err)
		}
	})
}

func newAudienceSwitchSessionService(t *testing.T, tdb *testutil.TestDB) *session.Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}

	return session.New(session.SessionConfig{
		Store:                tdb.Store,
		Tx:                   tdb.Tx,
		AccessTokens:         token.NewAccessTokenService(keySet, "authara-test", 10*time.Minute),
		SessionTTL:           time.Hour,
		RefreshTokenTTL:      time.Hour,
		RefreshTokenRotation: 0,
		Organizations: organization.New(organization.Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		}),
	})
}

func addSessionCookies(r *http.Request, accessToken string, refreshToken string) {
	recorder := httptest.NewRecorder()
	session.SetAccessToken(recorder, accessToken, 600)
	session.SetRefreshToken(recorder, refreshToken, 3600)
	for _, cookie := range recorder.Result().Cookies() {
		r.AddCookie(cookie)
	}
}

func switchedSessionCookies(t *testing.T, recorder *httptest.ResponseRecorder) (string, string) {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range recorder.Result().Cookies() {
		r.AddCookie(cookie)
	}
	accessToken, ok := session.ReadAccessToken(r)
	if !ok || accessToken == "" {
		t.Fatal("expected switched access token cookie")
	}
	refreshToken, ok := session.ReadRefreshToken(r)
	if !ok || refreshToken == "" {
		t.Fatal("expected switched refresh token cookie")
	}
	return accessToken, refreshToken
}
