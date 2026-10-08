package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/apple"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

type fakeAppleClient struct {
	result  apple.ExchangeResult
	err     error
	nonce   string
	revoked string
}

func (f *fakeAppleClient) Exchange(_ context.Context, _ string, nonce string) (apple.ExchangeResult, error) {
	f.nonce = nonce
	return f.result, f.err
}

func (f *fakeAppleClient) Revoke(_ context.Context, token string) error {
	f.revoked = token
	return f.err
}

type fakeAppleCredentials struct {
	userID      uuid.UUID
	token       string
	queuedToken string
	stagedLink  uuid.UUID
	stagedToken string
	stagedUntil time.Time
	saveErr     error
	queueErr    error
	stageErr    error
}

func (f *fakeAppleCredentials) Save(_ context.Context, userID uuid.UUID, token string) error {
	f.userID = userID
	f.token = token
	return f.saveErr
}

func (f *fakeAppleCredentials) QueueRevocation(_ context.Context, token string) error {
	f.queuedToken = token
	return f.queueErr
}

func (f *fakeAppleCredentials) StageProviderLink(_ context.Context, linkID uuid.UUID, token string, expiresAt time.Time) error {
	f.stagedLink = linkID
	f.stagedToken = token
	f.stagedUntil = expiresAt
	return f.stageErr
}

func (f *fakeAppleCredentials) PromoteProviderLink(_ context.Context, userID uuid.UUID, linkID uuid.UUID) error {
	if f.stageErr != nil {
		return f.stageErr
	}
	if linkID != f.stagedLink || f.stagedToken == "" {
		return errors.New("pending Apple credential not staged")
	}
	f.userID = userID
	f.token = f.stagedToken
	return nil
}

func TestAppleOptionsReturnStateNonceAndCookie(t *testing.T) {
	h := &APIHandler{OAuthProviders: appleTestProviders(), Apple: &fakeAppleClient{}}
	request := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple/options", nil)
	response, err := h.GetAppleLoginOptions(contractCtx(request.Context(), request), contract.GetAppleLoginOptionsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var options contract.AppleLoginOptions
	if err := json.Unmarshal(recorder.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.ClientId != "com.example.web" || options.State == "" || options.Nonce == "" || options.State == options.Nonce {
		t.Fatalf("unexpected options: %+v", options)
	}
	if !hasCookieValue(recorder.Result().Cookies(), "authara_apple_oauth", options.State+"."+options.Nonce) {
		t.Fatal("expected matching Apple flow cookie")
	}
}

func TestAppleOptionsReturnNotFoundWhenDisabled(t *testing.T) {
	h := &APIHandler{}
	request := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/apple/options", nil)
	response, err := h.GetAppleLoginOptions(contractCtx(request.Context(), request), contract.GetAppleLoginOptionsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAppleLoginRejectsStateMismatchBeforeExchange(t *testing.T) {
	securityEvents := &capturingSecurityEvents{}
	client := &fakeAppleClient{}
	h := &APIHandler{
		Auth: auth.New(auth.Config{SecurityEvents: securityEvents}), Apple: client,
		OAuthProviders: appleTestProviders(),
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
	request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "expected-state.expected-nonce"})
	response, err := h.LoginWithApple(contractCtx(request.Context(), request), contract.LoginWithAppleRequestObject{
		Body: &contract.AppleAuthorizationRequest{Code: "code", State: "wrong-state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusUnauthorized || client.nonce != "" {
		t.Fatalf("status = %d, exchange nonce = %q", recorder.Code, client.nonce)
	}
	if len(securityEvents.login) != 1 || securityEvents.login[0].AuthenticationMethod != domain.AuthenticationMethodApple {
		t.Fatalf("unexpected denial event: %+v", securityEvents.login)
	}
}

func TestAppleLoginRequiresVerifiedEmail(t *testing.T) {
	securityEvents := &capturingSecurityEvents{}
	client := &fakeAppleClient{result: apple.ExchangeResult{
		Identity: apple.Identity{OAuthID: "apple-subject"}, RefreshToken: "unused-refresh-token",
	}}
	h := &APIHandler{
		Auth: auth.New(auth.Config{SecurityEvents: securityEvents}), Apple: client,
		OAuthProviders: appleTestProviders(),
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
	request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
	response, err := h.LoginWithApple(contractCtx(request.Context(), request), contract.LoginWithAppleRequestObject{
		Body: &contract.AppleAuthorizationRequest{Code: "code", State: "state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusUnauthorized || client.revoked != "unused-refresh-token" {
		t.Fatalf("status = %d, revoked = %q", recorder.Code, client.revoked)
	}
	if !hasClearedCookie(recorder.Result().Cookies(), "authara_apple_oauth") {
		t.Fatal("expected Apple flow cookie to be consumed")
	}
}

func TestAppleLoginConsumesStateWhenExchangeFails(t *testing.T) {
	client := &fakeAppleClient{err: context.DeadlineExceeded}
	h := &APIHandler{
		Auth: auth.New(auth.Config{}), Apple: client,
		OAuthProviders: appleTestProviders(),
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
	request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
	response, err := h.LoginWithApple(contractCtx(request.Context(), request), contract.LoginWithAppleRequestObject{
		Body: &contract.AppleAuthorizationRequest{Code: "rejected-code", State: "state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusUnauthorized || !hasClearedCookie(recorder.Result().Cookies(), "authara_apple_oauth") {
		t.Fatalf("status = %d, cookies=%+v", recorder.Code, recorder.Result().Cookies())
	}
}

func TestAppleLoginCreatesSessionAndStoresRefreshToken(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		credentials := &fakeAppleCredentials{}
		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity:     apple.Identity{OAuthID: "apple-subject", Email: "apple@example.com", EmailVerified: true},
			RefreshToken: "refresh-token",
		}}
		h := newAppleAPIHandler(t, tdb, client, credentials)
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil).WithContext(ctx)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		response, err := h.LoginWithApple(contractCtx(ctx, request), contract.LoginWithAppleRequestObject{
			Body: &contract.AppleAuthorizationRequest{Code: "valid-code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		if client.nonce != "nonce" || credentials.token != "refresh-token" || credentials.userID == uuid.Nil {
			t.Fatalf("exchange/store mismatch: nonce=%q credentials=%+v", client.nonce, credentials)
		}
		if !hasCookie(recorder.Result().Cookies(), "authara_access") || !hasCookie(recorder.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected session cookies")
		}
	})
}

func TestAppleLoginDoesNotMergeAnExistingEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-collision@example.com", Username: "apple-collision"})
		if err != nil {
			t.Fatal(err)
		}
		passwordHash := "password-hash"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &passwordHash}); err != nil {
			t.Fatal(err)
		}
		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity:     apple.Identity{OAuthID: "different-apple-subject", Email: user.Email, EmailVerified: true},
			RefreshToken: "unused-refresh-token",
		}}
		credentials := &fakeAppleCredentials{}
		h := newAppleAPIHandler(t, tdb, client, credentials)
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		response, err := h.LoginWithApple(contractCtx(ctx, request), contract.LoginWithAppleRequestObject{
			Body: &contract.AppleAuthorizationRequest{Code: "code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusConflict || credentials.stagedToken != "unused-refresh-token" || credentials.queuedToken != "" {
			t.Fatalf("status = %d, staged=%q, queued=%q, body=%s", recorder.Code, credentials.stagedToken, credentials.queuedToken, recorder.Body.String())
		}
		recoveryURL, err := url.Parse(recorder.Header().Get("X-Authara-Redirect"))
		if err != nil {
			t.Fatal(err)
		}
		linkID, err := uuid.Parse(recoveryURL.Query().Get("link_id"))
		if err != nil {
			t.Fatalf("invalid recovery redirect %q: %v", recoveryURL.String(), err)
		}
		link, err := h.Auth.GetPendingProviderLink(ctx, linkID)
		if err != nil {
			t.Fatal(err)
		}
		if link.UserID != user.ID || link.Provider != domain.ProviderApple || link.ProviderUserID == nil || *link.ProviderUserID != "different-apple-subject" {
			t.Fatalf("unexpected recovery link: %+v", link)
		}
		if credentials.stagedLink != link.ID || !credentials.stagedUntil.Equal(link.ExpiresAt) {
			t.Fatalf("credential was not staged for the pending link: %+v", credentials)
		}
		if _, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderApple, "different-apple-subject"); err == nil {
			t.Fatal("Apple subject was silently linked to the existing account")
		}
	})
}

func TestAppleLoginSignsInAnExistingAppleIdentity(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "existing-apple@example.com", Username: "existing-apple"})
		if err != nil {
			t.Fatal(err)
		}
		providerUserID := "existing-apple-subject"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &providerUserID,
		}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity:     apple.Identity{OAuthID: providerUserID, Email: user.Email, EmailVerified: true},
			RefreshToken: "current-refresh-token",
		}}
		credentials := &fakeAppleCredentials{}
		h := newAppleAPIHandler(t, tdb, client, credentials)
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		response, err := h.LoginWithApple(contractCtx(ctx, request), contract.LoginWithAppleRequestObject{
			Body: &contract.AppleAuthorizationRequest{Code: "code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}

		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		if credentials.userID != user.ID || credentials.token != "current-refresh-token" {
			t.Fatalf("credentials = %+v", credentials)
		}
		if recorder.Header().Get("X-Authara-Redirect") != "" {
			t.Fatal("existing Apple identity was sent through account recovery")
		}
	})
}

func TestAppleProofCompletesGoogleAccountRecovery(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-proof@example.com", Username: "apple-proof"})
		if err != nil {
			t.Fatal(err)
		}
		appleSubject := "existing-apple-proof-subject"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &appleSubject,
		}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		providers := oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
			oauth.NewOAuthProvider(domain.ProviderGoogle, "google-client", "https://auth.example.com"),
			oauth.NewOAuthProvider(domain.ProviderApple, "com.example.web", "https://auth.example.com"),
		}}
		credentials := &fakeAppleCredentials{}
		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity: apple.Identity{OAuthID: appleSubject}, RefreshToken: "fresh-apple-token",
		}}
		organizations := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
		h := &APIHandler{
			Auth: auth.New(auth.Config{
				Store: tdb.Store, Tx: tdb.Tx, OAuthProviders: providers,
				Organizations: organizations, AppleCredentials: credentials,
			}),
			Session: newAPIHandlerTestSessionService(t, tdb), OAuthProviders: providers,
			Apple: client, AppleCredentials: credentials, AccessTTL: time.Minute, RefreshTTL: time.Hour,
		}

		recovery, code, message, ok := h.startAccountRecoveryLink(ctx, &google.Identity{
			OAuthID: "new-google-subject", Email: user.Email, EmailVerified: true,
		})
		if !ok {
			t.Fatalf("start recovery: code=%s message=%s", code, message)
		}
		if len(recovery.ProofMethods) != 1 || recovery.ProofMethods[0] != contract.AccountRecoveryLinkProofMethodsApple {
			t.Fatalf("proof methods = %+v", recovery.ProofMethods)
		}

		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/provider-links/recovery/"+recovery.LinkId.String()+"/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		responseObject, err := h.CompleteAccountRecoveryLinkWithApple(
			contractCtx(ctx, request),
			contract.CompleteAccountRecoveryLinkWithAppleRequestObject{
				LinkID: recovery.LinkId,
				Body:   &contract.AccountRecoveryAppleProofRequest{Code: "code", State: "state"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, responseObject)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		if !hasCookie(recorder.Result().Cookies(), "authara_access") || !hasCookie(recorder.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected session cookies")
		}
		if !hasClearedCookie(recorder.Result().Cookies(), "authara_apple_oauth") {
			t.Fatal("expected Apple flow cookie to be consumed")
		}
		if credentials.userID != user.ID || credentials.token != "fresh-apple-token" {
			t.Fatalf("credentials = %+v", credentials)
		}
		linked, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderGoogle, "new-google-subject")
		if err != nil || linked.UserID != user.ID {
			t.Fatalf("linked provider = %+v, err=%v", linked, err)
		}
	})
}

func TestAppleLoginRollsBackAccountWhenCredentialStorageFails(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	t.Cleanup(func() { _ = tdb.Store.Close() })
	ctx := context.Background()
	credentials := &fakeAppleCredentials{saveErr: errors.New("storage unavailable")}
	client := &fakeAppleClient{result: apple.ExchangeResult{
		Identity: apple.Identity{
			OAuthID: "apple-storage-failure", Email: "apple-storage-failure@example.com", EmailVerified: true,
		},
		RefreshToken: "refresh-token-to-revoke",
	}}
	h := newAppleAPIHandler(t, tdb, client, credentials)
	request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/apple", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
	response, err := h.LoginWithApple(contractCtx(ctx, request), contract.LoginWithAppleRequestObject{
		Body: &contract.AppleAuthorizationRequest{Code: "valid-code", State: "state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	writeContractResponse(t, recorder, response)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if credentials.queuedToken != "refresh-token-to-revoke" {
		t.Fatalf("queued token = %q", credentials.queuedToken)
	}
	if _, err := tdb.Store.GetUserByEmail(ctx, "apple-storage-failure@example.com"); !errors.Is(err, store.ErrUserNotFound) {
		t.Fatalf("account mutation was not rolled back: %v", err)
	}
}

func TestAppleLinkUsesSubjectWithoutRequiringAppleEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-link@example.com", Username: "apple-link"})
		if err != nil {
			t.Fatal(err)
		}
		passwordHash := "password-hash"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &passwordHash}); err != nil {
			t.Fatal(err)
		}
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		userSession, err := tdb.Store.CreateSession(ctx, domain.Session{UserID: user.ID, ActiveOrganizationID: organization.ID, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity: apple.Identity{OAuthID: "linked-without-email"}, RefreshToken: "linked-refresh-token",
		}}
		credentials := &fakeAppleCredentials{}
		h := newAppleAPIHandler(t, tdb, client, credentials)
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/account/auth-methods/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		requestContext := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), userSession.ID)
		response, err := h.LinkCurrentUserApple(contractCtx(requestContext, request), contract.LinkCurrentUserAppleRequestObject{
			Body: &contract.AppleAuthorizationRequest{Code: "code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusNoContent || credentials.token != "linked-refresh-token" {
			t.Fatalf("status = %d, stored=%q, body=%s", recorder.Code, credentials.token, recorder.Body.String())
		}
		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderApple, user.ID)
		if err != nil || provider.ProviderUserID == nil || *provider.ProviderUserID != "linked-without-email" {
			t.Fatalf("linked provider = %+v, %v", provider, err)
		}
	})
}

func TestAppleReauthenticationRequiresLinkedSubject(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-reauth@example.com", Username: "apple-reauth"})
		if err != nil {
			t.Fatal(err)
		}
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		linkedSubject := "linked-apple-subject"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &linkedSubject}); err != nil {
			t.Fatal(err)
		}
		userSession, err := tdb.Store.CreateSession(ctx, domain.Session{UserID: user.ID, ActiveOrganizationID: organization.ID, ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}

		client := &fakeAppleClient{result: apple.ExchangeResult{
			Identity:     apple.Identity{OAuthID: "different-subject", Email: user.Email, EmailVerified: true},
			RefreshToken: "unused-refresh-token",
		}}
		credentials := &fakeAppleCredentials{}
		h := newAppleAPIHandler(t, tdb, client, credentials)
		challenge, err := h.Session.StartAuthenticationChallenge(ctx, user.ID, userSession.ID, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/reauthenticate/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		requestContext := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), userSession.ID)
		response, err := h.ReauthenticateWithApple(contractCtx(requestContext, request), contract.ReauthenticateWithAppleRequestObject{
			Body: &contract.AppleReauthenticationRequest{AuthenticationChallengeId: challenge.ID, Code: "code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusUnauthorized || credentials.queuedToken != "unused-refresh-token" {
			t.Fatalf("status = %d, queued=%q", recorder.Code, credentials.queuedToken)
		}

		client.result.Identity.OAuthID = linkedSubject
		client.result.RefreshToken = "current-refresh-token"
		request = httptest.NewRequest(http.MethodPost, "/auth/api/v1/reauthenticate/apple", nil)
		request.AddCookie(&http.Cookie{Name: "authara_apple_oauth", Value: "state.nonce"})
		response, err = h.ReauthenticateWithApple(contractCtx(requestContext, request), contract.ReauthenticateWithAppleRequestObject{
			Body: &contract.AppleReauthenticationRequest{AuthenticationChallengeId: challenge.ID, Code: "code", State: "state"},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder = httptest.NewRecorder()
		writeContractResponse(t, recorder, response)
		if recorder.Code != http.StatusNoContent || credentials.token != "current-refresh-token" {
			t.Fatalf("status = %d, stored=%q, body=%s", recorder.Code, credentials.token, recorder.Body.String())
		}
	})
}

func newAppleAPIHandler(t *testing.T, tdb *testutil.TestDB, client AppleClient, credentials *fakeAppleCredentials) *APIHandler {
	t.Helper()
	providers := appleTestProviders()
	organizations := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
	return &APIHandler{
		Auth: auth.New(auth.Config{
			Store: tdb.Store, Tx: tdb.Tx, OAuthProviders: providers,
			Organizations: organizations, AppleCredentials: credentials,
		}),
		Session: newAPIHandlerTestSessionService(t, tdb), OAuthProviders: providers,
		Apple: client, AppleCredentials: credentials, AccessTTL: time.Minute, RefreshTTL: time.Hour,
	}
}

func appleTestProviders() oauth.OAuthProviders {
	return oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderApple, "com.example.web", "https://auth.example.com"),
	}}
}

func hasClearedCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.MaxAge < 0 {
			return true
		}
	}
	return false
}
