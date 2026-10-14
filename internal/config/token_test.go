package config

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func TestTokenParsesEd25519Seeds(t *testing.T) {
	seed := bytes.Repeat([]byte{1}, ed25519.SeedSize)
	cfg := Token{
		Issuer:                "authara",
		ActiveKeyID:           "active",
		PrivateKeys:           map[string]string{"active": base64.StdEncoding.EncodeToString(seed)},
		AccessTokenTTLMinutes: 10,
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := cfg.parse(); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.KeySet == nil || cfg.KeySet.ActiveKeyID() != "active" {
		t.Fatalf("unexpected key set: %+v", cfg.KeySet)
	}
}

func TestTokenRejectsInvalidEd25519Seeds(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "invalid base64", value: "!!!", want: "is not valid base64"},
		{name: "short seed", value: base64.StdEncoding.EncodeToString([]byte("short")), want: "must decode to exactly 32 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Token{ActiveKeyID: "active", PrivateKeys: map[string]string{"active": tt.value}}
			if err := cfg.parse(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTokenRejectsBlankIssuerAndActiveKeyID(t *testing.T) {
	seed := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	for _, cfg := range []Token{
		{Issuer: " ", ActiveKeyID: "active", PrivateKeys: map[string]string{"active": seed}, AccessTokenTTLMinutes: 10},
		{Issuer: "authara", ActiveKeyID: " ", PrivateKeys: map[string]string{" ": seed}, AccessTokenTTLMinutes: 10},
	} {
		if err := cfg.validate(); err == nil {
			t.Fatalf("validate(%+v) unexpectedly succeeded", cfg)
		}
	}
}
