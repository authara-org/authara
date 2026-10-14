package token

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func testSeed(fill byte) []byte {
	return bytes.Repeat([]byte{fill}, ed25519.SeedSize)
}

func TestNewKeySet_Succeeds(t *testing.T) {
	seeds := map[string][]byte{
		"key-1": testSeed(1),
		"key-2": testSeed(2),
	}

	ks, err := NewKeySet("key-1", seeds)
	if err != nil {
		t.Fatalf("NewKeySet returned error: %v", err)
	}
	if ks.ActiveKeyID() != "key-1" {
		t.Fatalf("active key id = %q, want key-1", ks.ActiveKeyID())
	}

	gotID, gotKey := ks.SigningKey()
	if gotID != "key-1" {
		t.Fatalf("signing key id = %q, want key-1", gotID)
	}
	wantKey := ed25519.NewKeyFromSeed(seeds["key-1"])
	if !bytes.Equal(gotKey, wantKey) {
		t.Fatal("signing key was not derived from the configured seed")
	}
}

func TestNewKeySet_RejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		active string
		seeds  map[string][]byte
		want   string
	}{
		{name: "empty active id", seeds: map[string][]byte{"key-1": testSeed(1)}, want: "active key id must be set"},
		{name: "blank active id", active: " ", seeds: map[string][]byte{"key-1": testSeed(1)}, want: "active key id must be set"},
		{name: "missing active key", active: "missing", seeds: map[string][]byte{"key-1": testSeed(1)}, want: "active signing key not found"},
		{name: "empty key id", active: "key-1", seeds: map[string][]byte{"key-1": testSeed(1), "": testSeed(2)}, want: "key id must not be empty"},
		{name: "blank key id", active: "key-1", seeds: map[string][]byte{"key-1": testSeed(1), " ": testSeed(2)}, want: "key id must not be empty"},
		{name: "short seed", active: "key-1", seeds: map[string][]byte{"key-1": []byte("short")}, want: "signing key \"key-1\" must be a 32-byte Ed25519 seed"},
		{name: "nil seed", active: "key-1", seeds: map[string][]byte{"key-1": nil}, want: "signing key \"key-1\" must be a 32-byte Ed25519 seed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks, err := NewKeySet(tt.active, tt.seeds)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("NewKeySet error = %v, want %q", err, tt.want)
			}
			if ks != nil {
				t.Fatal("NewKeySet returned a key set on error")
			}
		})
	}
}

func TestKeySet_ReturnsCopies(t *testing.T) {
	ks, err := NewKeySet("key-1", map[string][]byte{"key-1": testSeed(1)})
	if err != nil {
		t.Fatal(err)
	}

	_, signingKey := ks.SigningKey()
	verificationKey, ok := ks.VerificationKey("key-1")
	if !ok {
		t.Fatal("verification key not found")
	}
	signingKey[0] ^= 0xff
	verificationKey[0] ^= 0xff

	_, signingKeyAgain := ks.SigningKey()
	verificationKeyAgain, _ := ks.VerificationKey("key-1")
	if bytes.Equal(signingKey, signingKeyAgain) || bytes.Equal(verificationKey, verificationKeyAgain) {
		t.Fatal("callers can mutate key-set storage")
	}
}

func TestVerificationKey_NotFound(t *testing.T) {
	ks, err := NewKeySet("key-1", map[string][]byte{"key-1": testSeed(1)})
	if err != nil {
		t.Fatal(err)
	}

	key, ok := ks.VerificationKey("missing")
	if ok || key != nil {
		t.Fatalf("missing key = %v, %t; want nil, false", key, ok)
	}
}

func TestJWKSContainsOnlySortedPublicKeys(t *testing.T) {
	seeds := map[string][]byte{
		"key-b": testSeed(2),
		"key-a": testSeed(1),
	}
	ks, err := NewKeySet("key-b", seeds)
	if err != nil {
		t.Fatal(err)
	}

	jwks := ks.JWKS()
	if len(jwks.Keys) != 2 || jwks.Keys[0].KeyID != "key-a" || jwks.Keys[1].KeyID != "key-b" {
		t.Fatalf("JWKS keys = %+v, want key-a then key-b", jwks.Keys)
	}
	for _, key := range jwks.Keys {
		if key.KeyType != "OKP" || key.Curve != "Ed25519" || key.Algorithm != "EdDSA" || key.Use != "sig" {
			t.Fatalf("unexpected JWK metadata: %+v", key)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			t.Fatalf("invalid JWK x value %q: %v", key.X, err)
		}
		want := ed25519.NewKeyFromSeed(seeds[key.KeyID]).Public().(ed25519.PublicKey)
		if !bytes.Equal(decoded, want) {
			t.Fatalf("JWK %q does not contain the derived public key", key.KeyID)
		}
	}
}
