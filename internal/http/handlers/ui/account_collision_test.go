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
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	appleoauth "github.com/authara-org/authara/internal/oauth/apple"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

type fakeUIAppleClient struct {
	result  appleoauth.ExchangeResult
	err     error
	nonce   string
	revoked string
}

func (f *fakeUIAppleClient) Exchange(_ context.Context, _ string, nonce string) (appleoauth.ExchangeResult, error) {
	f.nonce = nonce
	return f.result, f.err
}

func (f *fakeUIAppleClient) Revoke(_ context.Context, token string) error {
	f.revoked = token
	return f.err
}

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
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
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

func TestProviderLinkConfirmPostLinksAppleAfterPasswordProof(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "apple-recovery@example.com",
			Username: "apple-recovery",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		passwordHash, err := authsvc.Hash("correct-password")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &passwordHash,
		}); err != nil {
			t.Fatal(err)
		}

		providers := oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
			oauth.NewOAuthProvider(domain.ProviderApple, "com.example.web", "https://auth.example.com"),
		}}
		appleCredentials, err := appleoauth.NewCredentials(
			tdb.Store,
			"test-key",
			map[string][]byte{"test-key": []byte("01234567890123456789012345678901")},
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		authService := authsvc.New(authsvc.Config{
			Store: tdb.Store, Tx: tdb.Tx, OAuthProviders: providers,
			Organizations:    organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}),
			AppleCredentials: appleCredentials,
		})
		link, err := authService.StartAccountRecoveryProviderLink(ctx, authsvc.OAuthIdentityInput{
			Provider:              domain.ProviderApple,
			Email:                 user.Email,
			ProviderUserID:        "apple-subject-to-link",
			ProviderEmailVerified: true,
		}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := appleCredentials.StageProviderLink(ctx, link.ID, "staged-refresh-token", link.ExpiresAt); err != nil {
			t.Fatal(err)
		}

		keySet, err := token.NewKeySet("test-key", map[string][]byte{
			"test-key": []byte("01234567890123456789012345678901"),
		})
		if err != nil {
			t.Fatal(err)
		}
		h := &UIHandler{
			Auth: authService,
			Session: session.New(session.SessionConfig{
				Store: tdb.Store,
				Tx:    tdb.Tx,
				AccessTokens: token.NewAccessTokenService(
					keySet,
					"authara-test",
					time.Minute,
				),
				SessionTTL:      time.Hour,
				RefreshTokenTTL: time.Hour,
				Organizations: organization.New(organization.Config{
					Store: tdb.Store, Tx: tdb.Tx,
				}),
			}),
			Limiter: ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
				LoginIPLimit: 10, LoginIPWindow: time.Hour,
				LoginEmailLimit: 10, LoginEmailWindow: time.Hour,
			}),
			AccessTTL: time.Minute, RefreshTTL: time.Hour,
			Render: render.New(render.Assets{}, false),
		}

		requestContext := httpctx.WithReturnTo(ctx, "/private")
		request := providerLinkConfirmRequest(requestContext, link.ID.String(), "correct-password")
		recorder := httptest.NewRecorder()
		h.ProviderLinkConfirmPost(recorder, request)

		if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/private" {
			t.Fatalf("status = %d, location = %q, body=%s", recorder.Code, recorder.Header().Get("Location"), recorder.Body.String())
		}
		linked, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderApple, "apple-subject-to-link")
		if err != nil {
			t.Fatal(err)
		}
		if linked.UserID != user.ID {
			t.Fatalf("Apple identity linked to user %s, want %s", linked.UserID, user.ID)
		}
		credential, err := tdb.Store.GetAppleCredentialByUserID(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if credential.AuthProviderID != linked.ID || string(credential.EncryptedRefreshToken) == "staged-refresh-token" {
			t.Fatalf("unexpected promoted Apple credential: %+v", credential)
		}
		if !hasResponseCookie(recorder.Result().Cookies(), "authara_access") || !hasResponseCookie(recorder.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected session cookies after account recovery")
		}
	})
}

func TestAccountCollisionProofOptionsIncludeAppleWithoutPassword(t *testing.T) {
	h := &UIHandler{Apple: &fakeUIAppleClient{}}
	options := h.accountCollisionProofOptions([]domain.AuthProvider{{Provider: domain.ProviderApple}})

	if len(options) != 1 || options[0].Provider != string(domain.ProviderApple) || options[0].Password {
		t.Fatalf("unexpected proof options: %+v", options)
	}
}

func TestAppleProviderProofLinksGoogleAndCreatesSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "apple-proof@example.com",
			Username: "apple-proof",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}
		appleSubject := "existing-apple-proof-subject"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &appleSubject,
		}); err != nil {
			t.Fatal(err)
		}

		providers := oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
			oauth.NewOAuthProvider(domain.ProviderApple, "com.example.web", "https://auth.example.com"),
			oauth.NewOAuthProvider(domain.ProviderGoogle, "google-client", "https://auth.example.com"),
		}}
		appleCredentials, err := appleoauth.NewCredentials(
			tdb.Store,
			"test-key",
			map[string][]byte{"test-key": []byte("01234567890123456789012345678901")},
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		authService := authsvc.New(authsvc.Config{
			Store: tdb.Store, Tx: tdb.Tx, OAuthProviders: providers,
			Organizations:    organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}),
			AppleCredentials: appleCredentials,
		})
		link, err := authService.StartAccountRecoveryProviderLink(ctx, authsvc.OAuthIdentityInput{
			Provider:              domain.ProviderGoogle,
			Email:                 user.Email,
			ProviderUserID:        "google-subject-to-link",
			ProviderEmailVerified: true,
		}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}

		keySet, err := token.NewKeySet("test-key", map[string][]byte{
			"test-key": []byte("01234567890123456789012345678901"),
		})
		if err != nil {
			t.Fatal(err)
		}
		appleClient := &fakeUIAppleClient{result: appleoauth.ExchangeResult{
			Identity: appleoauth.Identity{OAuthID: appleSubject}, RefreshToken: "proof-refresh-token",
		}}
		h := &UIHandler{
			Auth: authService,
			Session: session.New(session.SessionConfig{
				Store: tdb.Store,
				Tx:    tdb.Tx,
				AccessTokens: token.NewAccessTokenService(
					keySet,
					"authara-test",
					time.Minute,
				),
				SessionTTL:      time.Hour,
				RefreshTokenTTL: time.Hour,
				Organizations: organization.New(organization.Config{
					Store: tdb.Store, Tx: tdb.Tx,
				}),
			}),
			Apple: appleClient, AppleCredentials: appleCredentials,
			AccessTTL: time.Minute, RefreshTTL: time.Hour,
		}

		body := `{"code":"code","state":"state","link_id":"` + link.ID.String() + `"}`
		request := httptest.NewRequest(http.MethodPost, "/auth/oauth/apple/proof", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		request = request.WithContext(httpctx.WithReturnTo(ctx, "/private"))
		recorder := httptest.NewRecorder()
		h.AppleProviderProofPost(recorder, request)

		if recorder.Code != http.StatusOK || recorder.Header().Get("X-Authara-Redirect") != "/private" {
			t.Fatalf("status = %d, redirect = %q, body=%s", recorder.Code, recorder.Header().Get("X-Authara-Redirect"), recorder.Body.String())
		}
		if appleClient.nonce != "nonce" {
			t.Fatalf("Apple nonce = %q", appleClient.nonce)
		}
		linked, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderGoogle, "google-subject-to-link")
		if err != nil || linked.UserID != user.ID {
			t.Fatalf("linked Google provider = %+v, %v", linked, err)
		}
		credential, err := tdb.Store.GetAppleCredentialByUserID(ctx, user.ID)
		if err != nil || credential.AuthProviderID == uuid.Nil {
			t.Fatalf("saved Apple credential = %+v, %v", credential, err)
		}
		if !hasResponseCookie(recorder.Result().Cookies(), "authara_access") || !hasResponseCookie(recorder.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected session cookies after Apple proof")
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

func hasResponseCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}
