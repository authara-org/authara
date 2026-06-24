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
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
)

func TestCSRFGetReturnsTokenAndCookie(t *testing.T) {
	h := &APIHandler{}
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/csrf", nil)
	rr := httptest.NewRecorder()

	h.CSRFGet(rr, req)

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

		signupReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/signup", `{"email":" API-AUTH@example.com ","password":"password123"}`)
		signupRR := httptest.NewRecorder()
		h.SignupPost(signupRR, signupReq)

		if signupRR.Code != http.StatusCreated {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusCreated, signupRR.Code, signupRR.Body.String())
		}
		if !hasCookie(signupRR.Result().Cookies(), "authara_access") || !hasCookie(signupRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected signup to set session cookies")
		}
		assertResponseTokens(t, signupRR.Body.Bytes())

		loginReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/login", `{"email":"api-auth@example.com","password":"password123"}`)
		loginRR := httptest.NewRecorder()
		h.LoginPost(loginRR, loginReq)

		if loginRR.Code != http.StatusOK {
			t.Fatalf("expected login status %d, got %d body=%s", http.StatusOK, loginRR.Code, loginRR.Body.String())
		}
		if !hasCookie(loginRR.Result().Cookies(), "authara_access") || !hasCookie(loginRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected login to set session cookies")
		}
		assertResponseTokens(t, loginRR.Body.Bytes())
	})
}

func assertResponseTokens(t *testing.T, body []byte) {
	t.Helper()

	var got tokensResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if got.AccessToken == "" || got.RefreshToken == "" {
		t.Fatalf("expected access and refresh tokens in response, got %+v", got)
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
