package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeApple struct {
	t         *testing.T
	server    *httptest.Server
	mu        sync.Mutex
	signing   *rsa.PrivateKey
	published *rsa.PublicKey
	kid       string
	claims    jwt.MapClaims
	tokenCode int
	revoked   string
}

func newFakeApple(t *testing.T) *fakeApple {
	t.Helper()
	signing, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeApple{t: t, signing: signing, published: &signing.PublicKey, kid: "apple-key-1", tokenCode: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	f.claims = f.validClaims()
	return f
}

func (f *fakeApple) validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss": f.server.URL, "aud": "com.example.web", "sub": "apple-user-1",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(),
		"nonce": "expected-nonce", "email": "Person@Example.COM",
		"email_verified": "true", "is_private_email": false,
	}
}

func (f *fakeApple) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/keys":
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(f.kid, f.published)}})
	case "/token":
		if f.tokenCode != http.StatusOK {
			w.WriteHeader(f.tokenCode)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("code") != "valid-code" || r.Form.Get("redirect_uri") != "https://auth.example.com/auth/oauth/apple/callback" {
			f.t.Errorf("unexpected exchange form: %v", r.Form)
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, cloneClaims(f.claims))
		token.Header["kid"] = f.kid
		raw, err := token.SignedString(f.signing)
		if err != nil {
			f.t.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": raw, "refresh_token": "apple-refresh-token"})
	case "/revoke":
		_ = r.ParseForm()
		f.revoked = r.Form.Get("token")
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeApple) client(t *testing.T) *Client {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(Config{
		ClientID: "com.example.web", TeamID: "TEAM123", KeyID: "CLIENTKEY",
		PrivateKey: privateKey, RedirectURI: "https://auth.example.com/auth/oauth/apple/callback",
		HTTPClient: f.server.Client(), Issuer: f.server.URL, JWKSURL: f.server.URL + "/keys",
		TokenURL: f.server.URL + "/token", RevokeURL: f.server.URL + "/revoke",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestExchangeAndRevoke(t *testing.T) {
	fake := newFakeApple(t)
	client := fake.client(t)

	result, err := client.Exchange(context.Background(), "valid-code", "expected-nonce")
	if err != nil {
		t.Fatal(err)
	}
	if result.Identity.OAuthID != "apple-user-1" || result.Identity.Email != "person@example.com" || !result.Identity.EmailVerified || result.Identity.PrivateRelay {
		t.Fatalf("unexpected identity: %+v", result.Identity)
	}
	if result.RefreshToken != "apple-refresh-token" {
		t.Fatalf("refresh token = %q", result.RefreshToken)
	}
	if err := client.Revoke(context.Background(), result.RefreshToken); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.revoked != result.RefreshToken {
		t.Fatalf("revoked token = %q", fake.revoked)
	}
}

func TestExchangeRejectsInvalidIdentityClaims(t *testing.T) {
	tests := []struct {
		name  string
		alter func(jwt.MapClaims)
		want  string
	}{
		{name: "wrong issuer", alter: func(c jwt.MapClaims) { c["iss"] = "https://attacker.example" }, want: "different provider"},
		{name: "wrong audience", alter: func(c jwt.MapClaims) { c["aud"] = "another-client" }, want: "audience"},
		{name: "expired", alter: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, want: "expired"},
		{name: "wrong nonce", alter: func(c jwt.MapClaims) { c["nonce"] = "wrong" }, want: "nonce"},
		{name: "missing subject", alter: func(c jwt.MapClaims) { delete(c, "sub") }, want: "subject"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeApple(t)
			test.alter(fake.claims)
			_, err := fake.client(t).Exchange(context.Background(), "valid-code", "expected-nonce")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("Exchange error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExchangeRejectsInvalidSignatureAndAppleError(t *testing.T) {
	fake := newFakeApple(t)
	client := fake.client(t)
	if _, err := client.Exchange(context.Background(), "valid-code", "expected-nonce"); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	replacement, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fake.mu.Unlock()
		t.Fatal(err)
	}
	fake.signing = replacement
	fake.mu.Unlock()
	if _, err := client.Exchange(context.Background(), "valid-code", "expected-nonce"); err == nil {
		t.Fatal("expected invalid signature to be rejected")
	}

	fake = newFakeApple(t)
	fake.tokenCode = http.StatusBadRequest
	_, err = fake.client(t).Exchange(context.Background(), "valid-code", "expected-nonce")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("Exchange error = %v", err)
	}
}

func TestExchangeRefreshesRotatedSigningKeys(t *testing.T) {
	fake := newFakeApple(t)
	client := fake.client(t)
	if _, err := client.Exchange(context.Background(), "valid-code", "expected-nonce"); err != nil {
		t.Fatal(err)
	}

	fake.mu.Lock()
	fake.signing, _ = rsa.GenerateKey(rand.Reader, 2048)
	fake.published = &fake.signing.PublicKey
	fake.kid = "apple-key-2"
	fake.mu.Unlock()
	if _, err := client.Exchange(context.Background(), "valid-code", "expected-nonce"); err != nil {
		t.Fatalf("exchange after key rotation: %v", err)
	}
}

func TestClientSecretUsesAppleRequiredClaims(t *testing.T) {
	fake := newFakeApple(t)
	client := fake.client(t)
	raw, err := client.clientSecret()
	if err != nil {
		t.Fatal(err)
	}
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		return &client.privateKey.PublicKey, nil
	}, jwt.WithValidMethods([]string{"ES256"}), jwt.WithAudience(Issuer), jwt.WithIssuer("TEAM123"))
	if err != nil || !parsed.Valid {
		t.Fatalf("client secret invalid: %v", err)
	}
	if claims.Subject != "com.example.web" || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 5*time.Minute || parsed.Header["kid"] != "CLIENTKEY" {
		t.Fatalf("unexpected client secret claims/header: %+v %+v", claims, parsed.Header)
	}
}

func TestEncryptDecryptRejectsTamperingAndWrongProvider(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := encrypt(key, []byte("refresh-token"), []byte("provider-1"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := decrypt(key, ciphertext, []byte("provider-1"))
	if err != nil || string(plaintext) != "refresh-token" {
		t.Fatalf("decrypt = %q, %v", plaintext, err)
	}
	if _, err := decrypt(key, ciphertext, []byte("provider-2")); err == nil {
		t.Fatal("expected additional-data mismatch to fail")
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 1
	if _, err := decrypt(key, tampered, []byte("provider-1")); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
}

func jwk(kid string, publicKey *rsa.PublicKey) map[string]string {
	exponent := big.NewInt(int64(publicKey.E)).Bytes()
	return map[string]string{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(exponent),
	}
}

func cloneClaims(source jwt.MapClaims) jwt.MapClaims {
	copy := make(jwt.MapClaims, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func TestPostFormRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponseBytes+1)))
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client()}
	if err := client.postForm(context.Background(), server.URL, url.Values{}, nil); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("postForm error = %v", err)
	}
}
