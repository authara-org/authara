package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestGoogleOptionsGetReturnsClientIDNonceAndCookie(t *testing.T) {
	h := &APIHandler{OAuthProviders: googleTestProviders()}
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/oauth/google/options", nil)
	rr := httptest.NewRecorder()

	resp, err := h.GetGoogleLoginOptions(contractCtx(req.Context(), req), contract.GetGoogleLoginOptionsRequestObject{})
	if err != nil {
		t.Fatalf("GetGoogleLoginOptions failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var body contract.GoogleLoginOptions
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ClientId != "test-google-client-id" || body.Nonce == "" {
		t.Fatalf("unexpected response: %+v", body)
	}
	if !hasCookieValue(rr.Result().Cookies(), "authara_oauth_nonce", body.Nonce) {
		t.Fatal("expected matching OAuth nonce cookie")
	}
}

func TestGoogleLoginPostRejectsMismatchedNonce(t *testing.T) {
	recorder := &capturingSecurityEvents{}
	h := &APIHandler{
		Auth:           auth.New(auth.Config{SecurityEvents: recorder}),
		Google:         google.New("test-google-client-id"),
		OAuthProviders: googleTestProviders(),
	}
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/api/v1/oauth/google",
		strings.NewReader(`{"credential":"id-token","nonce":"wrong"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "authara_oauth_nonce", Value: "expected"})
	rr := httptest.NewRecorder()

	resp, err := h.LoginWithGoogle(contractCtx(req.Context(), req), contract.LoginWithGoogleRequestObject{
		Body: &contract.GoogleLoginRequest{Credential: "id-token", Nonce: "wrong"},
	})
	if err != nil {
		t.Fatalf("LoginWithGoogle failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
	if len(recorder.login) != 1 ||
		recorder.login[0].Outcome != domain.SecurityEventOutcomeDenied ||
		recorder.login[0].ReasonCode != domain.SecurityEventReasonInvalidAssertion ||
		recorder.login[0].AuthenticationMethod != domain.AuthenticationMethodGoogle ||
		recorder.login[0].UserID != nil {
		t.Fatalf("unexpected Google login denial event: %+v", recorder.login)
	}
}

type capturingSecurityEvents struct {
	securityevent.NoopRecorder
	login []securityevent.Authentication
}

func (r *capturingSecurityEvents) AuthenticationLogin(_ context.Context, event securityevent.Authentication) error {
	r.login = append(r.login, event)
	return nil
}

func TestCompleteGoogleLoginCreatesSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newGoogleAPIHandler(t, tdb)
		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/google", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		resp := h.contractGoogleLogin(ctx, req, &google.Identity{
			OAuthID:       "google-user-123",
			Email:         "google-api@example.com",
			EmailVerified: true,
		}, "app", make(http.Header))
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if !hasCookie(rr.Result().Cookies(), "authara_access") || !hasCookie(rr.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected Google login to set session cookies")
		}
		assertResponseTokens(t, rr.Body.Bytes())
	})
}

func TestCompleteGoogleLoginRequiresExplicitLinkForExistingEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newGoogleAPIHandler(t, tdb)
		if _, err := tdb.Store.CreateUser(ctx, domain.User{
			ID:       uuid.New(),
			Email:    "existing-google-api@example.com",
			Username: "existing-google-api",
		}); err != nil {
			t.Fatalf("create existing user: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/oauth/google", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		resp := h.contractGoogleLogin(ctx, req, &google.Identity{
			OAuthID:       "google-existing-123",
			Email:         "existing-google-api@example.com",
			EmailVerified: true,
		}, "app", make(http.Header))
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusConflict, rr.Code, rr.Body.String())
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Error.Code != string(codeAccountLinkRequired) {
			t.Fatalf("expected %q, got %q", codeAccountLinkRequired, body.Error.Code)
		}
	})
}

func TestGoogleReauthenticationRequiresIdentityLinkedToCurrentUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "google-reauth@example.com",
			Username: "google-reauth",
		})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		linkedOAuthID := "linked-google-user"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderGoogle, ProviderUserID: &linkedOAuthID,
		}); err != nil {
			t.Fatal(err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: time.Now().UTC().Add(time.Hour), UserAgent: "google-reauth",
		})
		if err != nil {
			t.Fatal(err)
		}

		h := newGoogleAPIHandler(t, tdb)
		nonce := "reauth-nonce"
		request := httptest.NewRequest(http.MethodPost, "/auth/api/v1/reauthenticate/google", nil)
		request.AddCookie(&http.Cookie{Name: "authara_oauth_nonce", Value: nonce})
		requestCtx := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), session.ID)
		requestCtx = contractCtx(requestCtx, request)
		authenticationChallenge, err := h.Session.StartAuthenticationChallenge(ctx, user.ID, session.ID, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}

		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: "different-google-user"}}
		responseObject, err := h.ReauthenticateWithGoogle(requestCtx, contract.ReauthenticateWithGoogleRequestObject{
			Body: &contract.GoogleReauthenticationRequest{
				AuthenticationChallengeId: authenticationChallenge.ID,
				Credential:                "id-token",
				Nonce:                     nonce,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		writeContractResponse(t, recorder, responseObject)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected unlinked identity status %d, got %d", http.StatusUnauthorized, recorder.Code)
		}

		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: linkedOAuthID}}
		responseObject, err = h.ReauthenticateWithGoogle(requestCtx, contract.ReauthenticateWithGoogleRequestObject{
			Body: &contract.GoogleReauthenticationRequest{
				AuthenticationChallengeId: authenticationChallenge.ID,
				Credential:                "id-token",
				Nonce:                     nonce,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder = httptest.NewRecorder()
		writeContractResponse(t, recorder, responseObject)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, recorder.Code, recorder.Body.String())
		}
		if err := h.Session.RequireRecentAuthentication(ctx, user.ID, session.ID, time.Now().UTC()); err != nil {
			t.Fatalf("session was not marked recently authenticated: %v", err)
		}
	})
}

func newGoogleAPIHandler(t *testing.T, tdb *testutil.TestDB) *APIHandler {
	t.Helper()
	providers := googleTestProviders()
	orgs := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
	return &APIHandler{
		Auth: auth.New(auth.Config{
			Store:          tdb.Store,
			Tx:             tdb.Tx,
			OAuthProviders: providers,
			Organizations:  orgs,
		}),
		Session:        newAPIHandlerTestSessionService(t, tdb),
		OAuthProviders: providers,
		AccessTTL:      time.Minute,
		RefreshTTL:     time.Hour,
	}
}

func googleTestProviders() oauth.OAuthProviders {
	return oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
	}}
}
