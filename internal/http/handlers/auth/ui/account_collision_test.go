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
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
)

func TestProviderLinkConfirmPostRateLimitsPasswordProof(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "provider-proof-limit@example.com",
			Username: "provider-proof-limit",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		passwordHash, err := authsvc.Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:       user.ID,
			Provider:     domain.ProviderPassword,
			PasswordHash: &passwordHash,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}

		authService := authsvc.New(authsvc.Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			OAuthProviders: oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
				{Name: domain.ProviderGoogle, ClientID: "test-client-id"},
			}},
		})

		link, err := authService.StartAccountRecoveryProviderLink(ctx, authsvc.OAuthIdentityInput{
			Provider:              domain.ProviderGoogle,
			Email:                 user.Email,
			ProviderUserID:        "google-user-id-for-rate-limit",
			ProviderEmailVerified: true,
		}, time.Now().UTC())
		if err != nil {
			t.Fatalf("StartAccountRecoveryProviderLink failed: %v", err)
		}

		h := &UIHandler{
			Auth: authService,
			Limiter: ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
				LoginIPLimit:     1,
				LoginIPWindow:    time.Hour,
				LoginEmailLimit:  10,
				LoginEmailWindow: time.Hour,
			}),
			Render: render.New(render.Assets{}, false),
		}

		first := providerLinkConfirmRequest(ctx, link.ID.String(), "wrong-password")
		firstRR := httptest.NewRecorder()
		h.ProviderLinkConfirmPost(firstRR, first)
		if firstRR.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected first request status %d, got %d body=%s", http.StatusUnprocessableEntity, firstRR.Code, firstRR.Body.String())
		}

		second := providerLinkConfirmRequest(ctx, link.ID.String(), "wrong-password")
		secondRR := httptest.NewRecorder()
		h.ProviderLinkConfirmPost(secondRR, second)
		if secondRR.Code != http.StatusTooManyRequests {
			t.Fatalf("expected second request status %d, got %d body=%s", http.StatusTooManyRequests, secondRR.Code, secondRR.Body.String())
		}
		if !strings.Contains(secondRR.Body.String(), "Too many attempts") {
			t.Fatalf("expected rate-limit message, got body=%s", secondRR.Body.String())
		}
	})
}

func providerLinkConfirmRequest(ctx context.Context, linkID string, password string) *http.Request {
	form := url.Values{}
	form.Set("link_id", linkID)
	form.Set("password", password)

	req := httptest.NewRequest(http.MethodPost, "/auth/provider-links/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(ctx)
}
