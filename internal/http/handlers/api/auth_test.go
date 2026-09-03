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
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
)

func TestCSRFGetReturnsTokenAndCookie(t *testing.T) {
	h := &APIHandler{}
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/csrf", nil)
	rr := httptest.NewRecorder()

	resp, err := h.GetCsrfToken(contractCtx(req.Context(), req), contract.GetCsrfTokenRequestObject{})
	if err != nil {
		t.Fatalf("GetCsrfToken failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var body struct {
		Token string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Token == "" {
		t.Fatal("expected csrf token in response")
	}
	if !hasCookieValue(rr.Result().Cookies(), "authara_csrf", body.Token) {
		t.Fatal("expected matching csrf cookie")
	}
}

func TestSignupAndLoginSetSessionCookies(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		orgs := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
		h := &APIHandler{
			Auth:       auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: orgs}),
			Session:    newAPIHandlerTestSessionService(t, tdb),
			Limiter:    ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		signupReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/signup/direct", `{"email":" API-AUTH@example.com ","password":"password123"}`)
		signupRR := httptest.NewRecorder()
		signupResp, err := h.SignupDirect(contractCtx(ctx, signupReq), contract.SignupDirectRequestObject{
			Body: signupRequest(" API-AUTH@example.com ", "password123", ""),
		})
		if err != nil {
			t.Fatalf("SignupDirect failed: %v", err)
		}
		writeContractResponse(t, signupRR, signupResp)

		if signupRR.Code != http.StatusCreated {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusCreated, signupRR.Code, signupRR.Body.String())
		}
		if !hasCookie(signupRR.Result().Cookies(), "authara_access") || !hasCookie(signupRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected signup to set session cookies")
		}
		assertResponseTokens(t, signupRR.Body.Bytes())

		loginReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/login", `{"identifier":"api-auth@example.com","password":"password123"}`)
		loginRR := httptest.NewRecorder()
		loginResp, err := h.LoginWithPassword(contractCtx(ctx, loginReq), contract.LoginWithPasswordRequestObject{
			Body: passwordLoginRequest("api-auth@example.com", "password123"),
		})
		if err != nil {
			t.Fatalf("LoginWithPassword failed: %v", err)
		}
		writeContractResponse(t, loginRR, loginResp)

		if loginRR.Code != http.StatusOK {
			t.Fatalf("expected login status %d, got %d body=%s", http.StatusOK, loginRR.Code, loginRR.Body.String())
		}
		if !hasCookie(loginRR.Result().Cookies(), "authara_access") || !hasCookie(loginRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected login to set session cookies")
		}
		assertResponseTokens(t, loginRR.Body.Bytes())
	})
}

func TestLoginWithPasswordAcceptsUsername(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := auth.Hash("password123")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-username-login@example.com",
			Username: "APIUsernameLogin",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:       user.ID,
			Provider:     domain.ProviderPassword,
			PasswordHash: &passwordHash,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		h := &APIHandler{
			Auth:                 auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx}),
			Session:              newAPIHandlerTestSessionService(t, tdb),
			Limiter:              ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
			UsernameLoginEnabled: true,
			AccessTTL:            time.Minute,
			RefreshTTL:           time.Hour,
		}

		req := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/login", `{"identifier":"apiusernamelogin","password":"password123"}`)
		rr := httptest.NewRecorder()
		resp, err := h.LoginWithPassword(contractCtx(ctx, req), contract.LoginWithPasswordRequestObject{
			Body: passwordLoginRequest("apiusernamelogin", "password123"),
		})
		if err != nil {
			t.Fatalf("LoginWithPassword failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected login status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if !hasCookie(rr.Result().Cookies(), "authara_access") || !hasCookie(rr.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected username login to set session cookies")
		}
		var body contract.AuthSession
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode login response: %v", err)
		}
		if body.User.Id != user.ID {
			t.Fatalf("expected user id %q, got %q", user.ID, body.User.Id)
		}
	})
}

func TestLoginWithPasswordRejectsUsernameWhenDisabled(t *testing.T) {
	h := &APIHandler{UsernameLoginEnabled: false}
	req := apiJSONRequest(context.Background(), http.MethodPost, "/auth/api/v1/login", `{"identifier":"disabled-username","password":"password123"}`)
	rr := httptest.NewRecorder()

	resp, err := h.LoginWithPassword(contractCtx(req.Context(), req), contract.LoginWithPasswordRequestObject{
		Body: passwordLoginRequest("disabled-username", "password123"),
	})
	if err != nil {
		t.Fatalf("LoginWithPassword failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
	assertErrorMessage(t, rr.Body.Bytes(), "Please provide a valid email address.")
}

func TestSignupWithInvitationCodeJoinsOrganization(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-invite-owner@example.com",
			Username: "api-invite-owner",
		})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}

		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeMulti,
			InvitationTTL: time.Hour,
		})
		invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "api-invited@example.com",
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		h := &APIHandler{
			Auth:       auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: orgs}),
			Session:    newAPIHandlerTestSessionService(t, tdb),
			Limiter:    ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		req := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/signup/direct", `{"email":"api-invited@example.com","password":"password123","invitation_code":"`+invite.RawToken+`"}`)
		rr := httptest.NewRecorder()
		resp, err := h.SignupDirect(contractCtx(ctx, req), contract.SignupDirectRequestObject{
			Body: signupRequest("api-invited@example.com", "password123", invite.RawToken),
		})
		if err != nil {
			t.Fatalf("SignupDirect failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusCreated, rr.Code, rr.Body.String())
		}

		user, err := tdb.Store.GetUserByEmail(ctx, "api-invited@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, user.ID); err != nil {
			t.Fatalf("expected invited organization membership: %v", err)
		}
	})
}

func TestSignupWithInvitationCodeRejectsWrongEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-wrong-invite-owner@example.com",
			Username: "api-wrong-invite-owner",
		})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}

		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeMulti,
			InvitationTTL: time.Hour,
		})
		invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "api-right-invited@example.com",
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		h := &APIHandler{
			Auth:       auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: orgs}),
			Session:    newAPIHandlerTestSessionService(t, tdb),
			Limiter:    ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
			AccessTTL:  time.Minute,
			RefreshTTL: time.Hour,
		}

		req := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/signup/direct", `{"email":"api-wrong-invited@example.com","password":"password123","invitation_code":"`+invite.RawToken+`"}`)
		rr := httptest.NewRecorder()
		resp, err := h.SignupDirect(contractCtx(ctx, req), contract.SignupDirectRequestObject{
			Body: signupRequest("api-wrong-invited@example.com", "password123", invite.RawToken),
		})
		if err != nil {
			t.Fatalf("SignupDirect failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
		assertErrorMessage(t, rr.Body.Bytes(), "Invitation code does not match this email.")
	})
}

func assertResponseTokens(t *testing.T, body []byte) {
	t.Helper()

	var got contract.Tokens
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if got.AccessToken == "" || got.RefreshToken == "" {
		t.Fatalf("expected access and refresh tokens in response, got %+v", got)
	}
}

func assertErrorMessage(t *testing.T, body []byte, want string) {
	t.Helper()

	var got struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if got.Error.Message != want {
		t.Fatalf("expected error message %q, got %q", want, got.Error.Message)
	}
}

func apiJSONRequest(ctx context.Context, method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func hasCookie(cookies []*http.Cookie, name string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}

func hasCookieValue(cookies []*http.Cookie, name string, value string) bool {
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value == value {
			return true
		}
	}
	return false
}
