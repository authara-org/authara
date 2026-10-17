package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/microsoft"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
)

type fakeMicrosoftVerifier struct {
	identity *microsoft.Identity
	err      error
	calls    *int
}

func (f fakeMicrosoftVerifier) Authority() string {
	return "https://login.microsoftonline.com/common"
}

func (f fakeMicrosoftVerifier) VerifyIDToken(_ context.Context, _, _ string) (*microsoft.Identity, error) {
	if f.calls != nil {
		*f.calls++
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.identity, nil
}

func TestMicrosoftVerificationIsRateLimitedBeforeProviderCall(t *testing.T) {
	calls := 0
	securityEvents := &capturingSecurityEvents{}
	h := &APIHandler{
		Auth: auth.New(auth.Config{SecurityEvents: securityEvents}),
		Limiter: ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
			LoginIPLimit: 1, LoginIPWindow: time.Minute,
		}),
		Microsoft:      fakeMicrosoftVerifier{err: errors.New("rejected"), calls: &calls},
		OAuthProviders: microsoftTestProviders(),
	}
	verify := func(nonce string) {
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/microsoft", nil)
		request.RemoteAddr = "192.0.2.1:1234"
		request.AddCookie(&http.Cookie{Name: "authara_oauth_nonce", Value: nonce})
		responseObject, err := h.LoginWithMicrosoft(contractCtx(request.Context(), request), contract.LoginWithMicrosoftRequestObject{
			Body: &contract.MicrosoftLoginRequest{Credential: "id-token", Nonce: nonce},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, responseObject)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
		}
	}
	verify("first")
	verify("second")
	if calls != 1 {
		t.Fatalf("provider verification calls = %d, want 1", calls)
	}
	if len(securityEvents.login) != 1 {
		t.Fatalf("login denial events = %d, want 1", len(securityEvents.login))
	}
}

func TestMicrosoftOptionsReturnClientAuthorityNonceAndCookie(t *testing.T) {
	h := &APIHandler{
		Microsoft:      fakeMicrosoftVerifier{},
		OAuthProviders: microsoftTestProviders(),
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/microsoft/options", nil)
	responseObject, err := h.GetMicrosoftLoginOptions(contractCtx(request.Context(), request), contract.GetMicrosoftLoginOptionsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, responseObject)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var options contract.MicrosoftLoginOptions
	if err := json.Unmarshal(recorder.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.ClientId != "test-microsoft-client-id" || options.Authority != "https://login.microsoftonline.com/common" || options.Nonce == "" {
		t.Fatalf("unexpected options: %+v", options)
	}
	if !hasCookieValue(recorder.Result().Cookies(), "authara_oauth_nonce", options.Nonce) {
		t.Fatal("expected matching OAuth nonce cookie")
	}
}

func TestMicrosoftLoginRejectsNonceMismatchBeforeVerification(t *testing.T) {
	securityEvents := &capturingSecurityEvents{}
	h := &APIHandler{
		Auth:           auth.New(auth.Config{SecurityEvents: securityEvents}),
		Microsoft:      fakeMicrosoftVerifier{err: errors.New("must not be called")},
		OAuthProviders: microsoftTestProviders(),
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/microsoft", nil)
	request.AddCookie(&http.Cookie{Name: "authara_oauth_nonce", Value: "expected"})
	responseObject, err := h.LoginWithMicrosoft(contractCtx(request.Context(), request), contract.LoginWithMicrosoftRequestObject{
		Body: &contract.MicrosoftLoginRequest{Credential: "id-token", Nonce: "wrong"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, responseObject)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if len(securityEvents.login) != 1 || securityEvents.login[0].AuthenticationMethod != domain.AuthenticationMethodMicrosoft {
		t.Fatalf("unexpected denial event: %+v", securityEvents.login)
	}
}

func TestMicrosoftLinkAndReauthenticationAcceptTokenTransportChallenges(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email: "microsoft-native-link@example.com", Username: "microsoft-native-link",
		})
		if err != nil {
			t.Fatal(err)
		}
		passwordHash := "password-hash"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &passwordHash,
		}); err != nil {
			t.Fatal(err)
		}
		organizationService := organization.New(organization.Config{
			Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle,
		})
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		userSession, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: organization.ID, ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		providers := microsoftTestProviders()
		h := &APIHandler{
			Auth: auth.New(auth.Config{
				Store: tdb.Store, Tx: tdb.Tx, OAuthProviders: providers, Organizations: organizationService,
			}),
			Session: newAPIHandlerTestSessionService(t, tdb),
			Microsoft: fakeMicrosoftVerifier{identity: &microsoft.Identity{
				OAuthID: "microsoft-native-link-id", Email: user.Email, EmailVerified: true,
			}},
			OAuthProviders:  providers,
			OAuthChallenges: oauth.NewChallengeService(tdb.Store, tdb.Tx),
		}
		challenge, err := h.OAuthChallenges.Create(ctx, domain.ProviderMicrosoft, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}

		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/account/auth-methods/microsoft", nil)
		requestCtx := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), userSession.ID)
		responseObject, err := h.LinkCurrentUserMicrosoft(contractCtx(requestCtx, request), contract.LinkCurrentUserMicrosoftRequestObject{
			Body: &contract.MicrosoftLoginRequest{
				ChallengeId: &challenge.ID,
				Credential:  "id-token",
				Nonce:       challenge.Nonce,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, responseObject)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, recorder.Code, recorder.Body.String())
		}

		authenticationChallenge, err := h.Session.StartAuthenticationChallenge(ctx, user.ID, userSession.ID, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		challenge, err = h.OAuthChallenges.Create(ctx, domain.ProviderMicrosoft, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		request = httptest.NewRequest(http.MethodPost, "/auth/api/v1/reauthenticate/microsoft", nil)
		reauthenticationResponse, err := h.ReauthenticateWithMicrosoft(contractCtx(requestCtx, request), contract.ReauthenticateWithMicrosoftRequestObject{
			Body: &contract.MicrosoftReauthenticationRequest{
				AuthenticationChallengeId: authenticationChallenge.ID,
				ChallengeId:               &challenge.ID,
				Credential:                "id-token",
				Nonce:                     challenge.Nonce,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder = httptest.NewRecorder()
		writeContractResponse(t, recorder, reauthenticationResponse)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected reauthentication status %d, got %d body=%s", http.StatusNoContent, recorder.Code, recorder.Body.String())
		}
	})
}

func microsoftTestProviders() oauth.OAuthProviders {
	return oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderMicrosoft, "test-microsoft-client-id", "http://localhost:3000"),
	}}
}
