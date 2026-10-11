package microsoft

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	testClientID = "microsoft-client-id"
	testTenantID = "11111111-2222-3333-4444-555555555555"
	testObjectID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

func TestVerifyIDToken(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+testTenantID+"/discovery/v2.0/keys" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
	}))
	defer server.Close()

	client, err := New(Config{ClientID: testClientID, TenantID: "organizations", LoginHost: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	nonce := "expected-nonce"
	raw := microsoftToken(t, privateKey, server.URL, map[string]any{
		"tid": testTenantID, "oid": testObjectID, "nonce": nonce, "ver": "2.0",
		"email": "Person@Example.com", "xms_edov": true,
	})

	identity, err := client.VerifyIDToken(context.Background(), raw, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if identity.OAuthID != testTenantID+":"+testObjectID || identity.Email != "person@example.com" || !identity.EmailVerified {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestVerifyIDTokenRejectsInvalidIdentityClaims(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
	}))
	defer server.Close()
	client, err := New(Config{ClientID: testClientID, TenantID: testTenantID, LoginHost: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		claims map[string]any
		nonce  string
		want   string
	}{
		{name: "nonce", nonce: "different", claims: map[string]any{"tid": testTenantID, "oid": testObjectID, "nonce": "expected", "ver": "2.0", "email": "person@example.com", "xms_edov": true}, want: "nonce"},
		{name: "email", nonce: "expected", claims: map[string]any{"tid": testTenantID, "oid": testObjectID, "nonce": "expected", "ver": "2.0"}, want: "usable email"},
		{name: "invalid email", nonce: "expected", claims: map[string]any{"tid": testTenantID, "oid": testObjectID, "nonce": "expected", "ver": "2.0", "email": "person@"}, want: "usable email"},
		{name: "object id", nonce: "expected", claims: map[string]any{"tid": testTenantID, "oid": "not-a-uuid", "nonce": "expected", "ver": "2.0", "verified_primary_email": "person@example.com"}, want: "object id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := microsoftToken(t, privateKey, server.URL, tc.claims)
			if _, err := client.VerifyIDToken(context.Background(), raw, tc.nonce); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("VerifyIDToken error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestVerifyIDTokenAcceptsUnverifiedEmail(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	key := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: "test-key", Algorithm: string(jose.RS256), Use: "sig"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}})
	}))
	defer server.Close()
	client, err := New(Config{ClientID: testClientID, TenantID: testTenantID, LoginHost: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		claims map[string]any
		want   string
	}{
		{name: "email claim", claims: map[string]any{"email": "Person@Example.com"}, want: "person@example.com"},
		{name: "preferred username", claims: map[string]any{"preferred_username": "Person@Example.com"}, want: "person@example.com"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{
				"tid": testTenantID, "oid": testObjectID, "nonce": "expected", "ver": "2.0",
			}
			for key, value := range tc.claims {
				claims[key] = value
			}
			identity, err := client.VerifyIDToken(context.Background(), microsoftToken(t, privateKey, server.URL, claims), "expected")
			if err != nil {
				t.Fatal(err)
			}
			if identity.Email != tc.want || identity.EmailVerified {
				t.Fatalf("identity = %#v", identity)
			}
		})
	}
}

func TestTenantPolicy(t *testing.T) {
	for _, tc := range []struct {
		configured string
		tenant     string
		want       bool
	}{
		{configured: "common", tenant: consumerTenantID, want: true},
		{configured: "organizations", tenant: testTenantID, want: true},
		{configured: "organizations", tenant: consumerTenantID, want: false},
		{configured: "consumers", tenant: consumerTenantID, want: true},
		{configured: "consumers", tenant: testTenantID, want: false},
		{configured: testTenantID, tenant: testTenantID, want: true},
	} {
		client, err := New(Config{ClientID: testClientID, TenantID: tc.configured})
		if err != nil {
			t.Fatal(err)
		}
		if got := client.allowsTenant(tc.tenant); got != tc.want {
			t.Errorf("allowsTenant(%q, %q) = %v, want %v", tc.configured, tc.tenant, got, tc.want)
		}
	}
}

func TestNewRejectsNonCanonicalTenantUUID(t *testing.T) {
	if _, err := New(Config{ClientID: testClientID, TenantID: "urn:uuid:" + testTenantID}); err == nil {
		t.Fatal("New accepted a non-canonical tenant UUID")
	}
}

func microsoftToken(t *testing.T, privateKey *rsa.PrivateKey, issuerHost string, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, (&jose.SignerOptions{}).WithHeader("kid", "test-key"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	standard := jwt.Claims{
		Issuer:   issuerHost + "/" + testTenantID + "/v2.0",
		Audience: jwt.Audience{testClientID},
		IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
	}
	raw, err := jwt.Signed(signer).Claims(standard).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
