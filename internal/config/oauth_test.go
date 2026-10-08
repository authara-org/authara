package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func validAppleOAuth(t *testing.T) OAuth {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return OAuth{
		Providers: []string{"apple"}, AppleClientID: "com.example.web",
		AppleTeamID: "TEAM123", AppleKeyID: "KEY123",
		ApplePrivateKeyBase64: base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		AppleTokenActiveKeyID: "primary",
		AppleTokenKeys:        map[string]string{"primary": base64.StdEncoding.EncodeToString(make([]byte, 32))},
	}
}

func TestAppleOAuthValidateAndParse(t *testing.T) {
	oauth := validAppleOAuth(t)
	if err := oauth.validate(); err != nil {
		t.Fatal(err)
	}
	if err := oauth.parse(); err != nil {
		t.Fatal(err)
	}
	if oauth.ApplePrivateKey == nil || len(oauth.AppleDecodedTokenKeys["primary"]) != 32 {
		t.Fatal("Apple keys were not parsed")
	}
}

func TestAppleOAuthRejectsIncompleteOrInvalidKeys(t *testing.T) {
	oauth := validAppleOAuth(t)
	oauth.AppleTeamID = ""
	if err := oauth.validate(); err == nil || !strings.Contains(err.Error(), "APPLE_TEAM_ID") {
		t.Fatalf("validate error = %v", err)
	}

	oauth = validAppleOAuth(t)
	oauth.AppleTokenKeys["primary"] = base64.StdEncoding.EncodeToString(make([]byte, 31))
	if err := oauth.parse(); err == nil || !strings.Contains(err.Error(), "exactly 32 bytes") {
		t.Fatalf("parse error = %v", err)
	}
}

func TestProductionAppleOAuthRequiresHTTPSPublicURL(t *testing.T) {
	cfg := validProdConfigForValidate()
	cfg.OAuth = validAppleOAuth(t)
	cfg.Values.PublicURL = "http://auth.example.com"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("validate error = %v", err)
	}
}
