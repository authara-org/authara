package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestRequireRecentAuthenticationAPIDistinguishesStaleSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
		sessionService := newAudienceSwitchSessionService(t, tdb)
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "recent-middleware@example.com", Username: "recent-middleware"})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}
		accessToken, _, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "test", now, "")
		if err != nil {
			t.Fatal(err)
		}
		identity, err := sessionService.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatal(err)
		}
		requestCtx := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), identity.SessionID)
		handler := RequireRecentAuthenticationAPI(sessionService, func() time.Time {
			return now.Add(11 * time.Minute)
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/account/password", nil).WithContext(requestCtx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusPreconditionRequired {
			t.Fatalf("stale response status = %d, want %d", rr.Code, http.StatusPreconditionRequired)
		}
		var body struct {
			Error                   response.Error `json:"error"`
			AuthenticationChallenge struct {
				ID        uuid.UUID `json:"id"`
				ExpiresAt time.Time `json:"expires_at"`
			} `json:"authentication_challenge"`
			ReauthenticateURL string `json:"reauthenticate_url"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != response.CodeRecentAuthenticationRequired {
			t.Fatalf("error code = %q", body.Error.Code)
		}
		if body.AuthenticationChallenge.ID == uuid.Nil || body.ReauthenticateURL == "" {
			t.Fatalf("missing authentication challenge: %+v", body)
		}

		freshAt := now.Add(11 * time.Minute)
		if err := sessionService.CompleteAuthenticationChallenge(ctx, user.ID, identity.SessionID, body.AuthenticationChallenge.ID, domain.AuthenticationMethodPassword, freshAt); err != nil {
			t.Fatal(err)
		}
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("fresh response status = %d, want %d", rr.Code, http.StatusNoContent)
		}
	})
}
