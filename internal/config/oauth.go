package config

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

type OAuth struct {
	Providers []string `env:"AUTHARA_OAUTH_PROVIDERS"`

	GoogleClientID string `env:"AUTHARA_OAUTH_GOOGLE_CLIENT_ID"`

	AppleClientID         string            `env:"AUTHARA_OAUTH_APPLE_CLIENT_ID"`
	AppleTeamID           string            `env:"AUTHARA_OAUTH_APPLE_TEAM_ID"`
	AppleKeyID            string            `env:"AUTHARA_OAUTH_APPLE_KEY_ID"`
	ApplePrivateKeyBase64 string            `env:"AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64"`
	AppleTokenActiveKeyID string            `env:"AUTHARA_OAUTH_APPLE_TOKEN_ACTIVE_KEY_ID"`
	AppleTokenKeys        map[string]string `env:"AUTHARA_OAUTH_APPLE_TOKEN_KEYS"`
	ApplePrivateKey       *ecdsa.PrivateKey
	AppleDecodedTokenKeys map[string][]byte
}

func (oa *OAuth) validate() error {
	seen := make(map[string]struct{})

	for _, raw := range oa.Providers {
		p := strings.ToLower(strings.TrimSpace(raw))

		if p == "" {
			continue
		}

		if _, ok := seen[p]; ok {
			return fmt.Errorf("duplicate OAuth provider %q", p)
		}
		seen[p] = struct{}{}

		switch p {
		case "google":
			if oa.GoogleClientID == "" {
				return fmt.Errorf("AUTHARA_OAUTH_GOOGLE_CLIENT_ID is required")
			}
		case "apple":
			if strings.TrimSpace(oa.AppleClientID) == "" {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_CLIENT_ID is required")
			}
			if strings.TrimSpace(oa.AppleTeamID) == "" {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_TEAM_ID is required")
			}
			if strings.TrimSpace(oa.AppleKeyID) == "" {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_KEY_ID is required")
			}
			if strings.TrimSpace(oa.ApplePrivateKeyBase64) == "" {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64 is required")
			}
			if strings.TrimSpace(oa.AppleTokenActiveKeyID) == "" {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_TOKEN_ACTIVE_KEY_ID is required")
			}
			if len(oa.AppleTokenKeys) == 0 {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_TOKEN_KEYS must contain at least one key")
			}
			if _, ok := oa.AppleTokenKeys[oa.AppleTokenActiveKeyID]; !ok {
				return fmt.Errorf("AUTHARA_OAUTH_APPLE_TOKEN_ACTIVE_KEY_ID %q not found in AUTHARA_OAUTH_APPLE_TOKEN_KEYS", oa.AppleTokenActiveKeyID)
			}
		default:
			return fmt.Errorf("unsupported OAuth provider %q", p)
		}
	}
	return nil
}

func (oa *OAuth) parse() error {
	if !oa.providerEnabled("apple") {
		return nil
	}

	encoded := strings.TrimSpace(oa.ApplePrivateKeyBase64)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64 is not valid base64")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return fmt.Errorf("AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64 does not contain a PEM private key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64 is not a PKCS#8 private key: %w", err)
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || privateKey.Curve.Params().Name != "P-256" {
		return fmt.Errorf("AUTHARA_OAUTH_APPLE_PRIVATE_KEY_BASE64 must contain an ECDSA P-256 private key")
	}
	oa.ApplePrivateKey = privateKey

	oa.AppleDecodedTokenKeys = make(map[string][]byte, len(oa.AppleTokenKeys))
	for id, value := range oa.AppleTokenKeys {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("AUTHARA_OAUTH_APPLE_TOKEN_KEYS[%q] is not valid base64", id)
		}
		if len(key) != 32 {
			return fmt.Errorf("AUTHARA_OAUTH_APPLE_TOKEN_KEYS[%q] must decode to exactly 32 bytes", id)
		}
		oa.AppleDecodedTokenKeys[id] = key
	}
	return nil
}

func (oa *OAuth) providerEnabled(name string) bool {
	for _, raw := range oa.Providers {
		if strings.EqualFold(strings.TrimSpace(raw), name) {
			return true
		}
	}
	return false
}
