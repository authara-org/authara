package config

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestChallengeParsesVerificationCodeKeysActiveFirst(t *testing.T) {
	active := bytes.Repeat([]byte{1}, 32)
	old := bytes.Repeat([]byte{2}, 32)
	cfg := Challenge{
		VerificationCodeActiveKeyID: "active",
		VerificationCodeKeys: map[string]string{
			"old":    base64.StdEncoding.EncodeToString(old),
			"active": base64.StdEncoding.EncodeToString(active),
		},
	}
	if err := cfg.parse(); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cfg.VerificationCodeSecrets) != 2 ||
		!bytes.Equal(cfg.VerificationCodeSecrets[0], active) ||
		!bytes.Equal(cfg.VerificationCodeSecrets[1], old) {
		t.Fatalf("verification secrets are not active-first: %v", cfg.VerificationCodeSecrets)
	}
}

func TestChallengeRejectsInvalidVerificationCodeKey(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "invalid base64", value: "!!!", want: "is not valid base64"},
		{name: "short key", value: base64.StdEncoding.EncodeToString([]byte("short")), want: "must decode to at least 32 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Challenge{
				VerificationCodeActiveKeyID: "active",
				VerificationCodeKeys:        map[string]string{"active": tt.value},
			}
			if err := cfg.parse(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestChallengeRejectsBlankVerificationCodeActiveKeyID(t *testing.T) {
	cfg := Challenge{
		TTL:                         time.Minute,
		VerificationCodeTTL:         time.Minute,
		MaxAttempts:                 1,
		MinResendInterval:           0,
		VerificationCodeActiveKeyID: " ",
		VerificationCodeKeys:        map[string]string{" ": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate unexpectedly succeeded")
	}
}
