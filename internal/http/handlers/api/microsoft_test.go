package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/microsoft"
)

type fakeMicrosoftVerifier struct {
	identity *microsoft.Identity
	err      error
}

func (f fakeMicrosoftVerifier) Authority() string {
	return "https://login.microsoftonline.com/common"
}

func (f fakeMicrosoftVerifier) VerifyIDToken(_ context.Context, _, _ string) (*microsoft.Identity, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.identity, nil
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

func microsoftTestProviders() oauth.OAuthProviders {
	return oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderMicrosoft, "test-microsoft-client-id", "http://localhost:3000"),
	}}
}
