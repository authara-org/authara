package google

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/api/idtoken"
)

func TestExchangeAuthorizationCode(t *testing.T) {
	var receivedCode string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse token request: %v", err)
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected token request: %s %q", r.Method, r.Header.Get("Content-Type"))
		}
		receivedCode = r.FormValue("code")
		if r.FormValue("client_id") != "client-id" || r.FormValue("client_secret") != "client-secret" || r.FormValue("redirect_uri") != "https://app.example" || r.FormValue("grant_type") != "authorization_code" {
			t.Fatalf("unexpected token form: %v", r.Form)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id_token":"google-id-token"}`)),
		}, nil
	})

	client := New("client-id", "client-secret")
	client.HTTPClient = &http.Client{Transport: transport}
	client.verifyToken = func(_ context.Context, token string, audience string) (*idtoken.Payload, error) {
		if token != "google-id-token" || audience != "client-id" {
			t.Fatalf("verify token = %q audience = %q", token, audience)
		}
		return &idtoken.Payload{Claims: map[string]any{
			"sub": "google-user", "email": "person@example.com", "email_verified": true,
		}}, nil
	}

	identity, err := client.ExchangeAuthorizationCode(context.Background(), "google-code", "https://app.example")
	if err != nil {
		t.Fatalf("exchange authorization code: %v", err)
	}
	if receivedCode != "google-code" || identity.OAuthID != "google-user" || identity.Email != "person@example.com" || !identity.EmailVerified {
		t.Fatalf("unexpected exchange result: code=%q identity=%+v", receivedCode, identity)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestExchangeAuthorizationCodeRejectsNonOriginRedirectURI(t *testing.T) {
	client := New("client-id", "client-secret")
	if _, err := client.ExchangeAuthorizationCode(context.Background(), "google-code", "https://app.example/callback"); err == nil {
		t.Fatal("expected redirect URI with a path to be rejected")
	}
}
