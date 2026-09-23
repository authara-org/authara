package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
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

func TestProviderLinkCompletionRechecksRecentAuthentication(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "provider-link-freshness@example.com", Username: "provider-link-freshness"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		authenticatedAt := now.Add(-2 * time.Minute)
		sessionRow, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: now.Add(time.Hour),
			AuthenticatedAt: &authenticatedAt, AuthenticationMethod: domain.AuthenticationMethodPassword,
		})
		if err != nil {
			t.Fatal(err)
		}
		h := &UIHandler{Session: session.New(session.SessionConfig{
			Store: tdb.Store, Tx: tdb.Tx, RecentAuthenticationWindow: time.Minute,
		})}
		requestCtx := httpctx.WithReturnTo(
			httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), sessionRow.ID),
			"/auth/account",
		)
		req := httptest.NewRequest(http.MethodPost, "/auth/oauth/google/callback", nil).WithContext(requestCtx)
		rr := httptest.NewRecorder()
		if h.requireRecentProviderLinkAuthentication(rr, req) {
			t.Fatal("stale provider-link completion was accepted")
		}
		if rr.Code != http.StatusPreconditionRequired {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusPreconditionRequired)
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			ReauthenticateURL       string `json:"reauthenticate_url"`
			AuthenticationChallenge struct {
				ID uuid.UUID `json:"id"`
			} `json:"authentication_challenge"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != "recent_authentication_required" || body.ReauthenticateURL == "" || body.AuthenticationChallenge.ID == uuid.Nil {
			t.Fatalf("unexpected stale response: %+v", body)
		}

		if err := h.Session.MarkRecentlyAuthenticated(ctx, user.ID, sessionRow.ID, domain.AuthenticationMethodPasskey, now); err != nil {
			t.Fatal(err)
		}
		rr = httptest.NewRecorder()
		if !h.requireRecentProviderLinkAuthentication(rr, req) {
			t.Fatalf("fresh provider-link completion was rejected with %d: %s", rr.Code, rr.Body.String())
		}
	})
}
