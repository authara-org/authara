package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	authsvc "github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/testutil"
)

func TestEmbeddedPasswordReauthenticationSignalsParentWithoutNavigation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "embedded-reauth@example.com",
			Username: "embedded-reauth",
		})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		passwordHash, err := authsvc.Hash("correct-password")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:       user.ID,
			Provider:     domain.ProviderPassword,
			PasswordHash: &passwordHash,
		}); err != nil {
			t.Fatal(err)
		}

		authenticatedAt := now.Add(-time.Hour)
		sessionRow, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: org.ID,
			ExpiresAt:            now.Add(time.Hour),
			AuthenticatedAt:      &authenticatedAt,
			AuthenticationMethod: domain.AuthenticationMethodPassword,
		})
		if err != nil {
			t.Fatal(err)
		}
		sessionService := session.New(session.SessionConfig{
			Store:                      tdb.Store,
			Tx:                         tdb.Tx,
			RecentAuthenticationWindow: 10 * time.Minute,
		})
		challenge, err := sessionService.StartAuthenticationChallenge(ctx, user.ID, sessionRow.ID, now)
		if err != nil {
			t.Fatal(err)
		}

		h := &UIHandler{
			Auth: authsvc.New(authsvc.Config{
				Store:          tdb.Store,
				Tx:             tdb.Tx,
				OAuthProviders: oauth.OAuthProviders{},
			}),
			Session: sessionService,
		}
		form := url.Values{
			"authentication_challenge_id": {challenge.ID.String()},
			"password":                    {"correct-password"},
			"embedded":                    {"1"},
		}
		req := httptest.NewRequest(http.MethodPost, "/auth/reauthenticate/password", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req = req.WithContext(httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), sessionRow.ID))
		rr := httptest.NewRecorder()

		h.ReauthenticatePasswordPost(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusNoContent, rr.Body.String())
		}
		if got := rr.Header().Get("HX-Trigger"); got != "autharaRecentAuthenticationComplete" {
			t.Fatalf("HX-Trigger = %q", got)
		}
		if err := sessionService.ValidateCompletedAuthenticationChallenge(ctx, user.ID, sessionRow.ID, challenge.ID); err != nil {
			t.Fatalf("challenge was not completed: %v", err)
		}
		if err := sessionService.RequireRecentAuthentication(ctx, user.ID, sessionRow.ID, time.Now().UTC()); err != nil {
			t.Fatalf("session was not marked recently authenticated: %v", err)
		}
	})
}
