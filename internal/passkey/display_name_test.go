package passkey

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func TestPasskeyRegistrationNameDefaults(t *testing.T) {
	tests := []struct {
		name       string
		metadata   RegistrationMetadata
		credential *webauthn.Credential
		createdAt  time.Time
		want       string
	}{
		{
			name: "mac touch id",
			metadata: RegistrationMetadata{
				PlatformHint: "macOS",
			},
			credential: platformCredential(protocol.Internal),
			createdAt:  time.Date(2026, time.May, 13, 12, 0, 0, 0, time.UTC),
			want:       "MacBook Pro • Touch ID • May 2026",
		},
		{
			name: "iphone face id",
			metadata: RegistrationMetadata{
				PlatformHint: "iOS",
				UserAgent:    "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)",
			},
			credential: platformCredential(protocol.Internal),
			createdAt:  time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC),
			want:       "iPhone • Face ID • Apr 2026",
		},
		{
			name: "windows hello",
			metadata: RegistrationMetadata{
				PlatformHint: "Windows",
			},
			credential: platformCredential(protocol.Internal),
			createdAt:  time.Date(2026, time.February, 2, 12, 0, 0, 0, time.UTC),
			want:       "Windows PC • Windows Hello • Feb 2026",
		},
		{
			name:       "security key",
			credential: crossPlatformCredential(protocol.USB),
			createdAt:  time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC),
			want:       "Security Key • Security Key • Jan 2026",
		},
		{
			name:       "known yubikey",
			credential: crossPlatformCredentialWithAAGUID("20ac7a17-c814-4833-93fe-539f0d5e3389", protocol.USB),
			createdAt:  time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC),
			want:       "YubiKey 5 Series • Security Key • Jan 2026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := passkeyRegistrationName(tt.metadata, tt.credential, tt.createdAt)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestPasskeyRegistrationNameKeepsExplicitName(t *testing.T) {
	got := passkeyRegistrationName(
		RegistrationMetadata{Name: " Work key "},
		platformCredential(protocol.Internal),
		time.Date(2026, time.May, 13, 12, 0, 0, 0, time.UTC),
	)
	if got != "Work key" {
		t.Fatalf("expected explicit name, got %q", got)
	}
}

func platformCredential(transports ...protocol.AuthenticatorTransport) *webauthn.Credential {
	return &webauthn.Credential{
		Transport: transports,
		Authenticator: webauthn.Authenticator{
			Attachment: protocol.Platform,
		},
	}
}

func crossPlatformCredential(transports ...protocol.AuthenticatorTransport) *webauthn.Credential {
	return &webauthn.Credential{
		Transport: transports,
		Authenticator: webauthn.Authenticator{
			Attachment: protocol.CrossPlatform,
		},
	}
}

func crossPlatformCredentialWithAAGUID(
	aaguid string,
	transports ...protocol.AuthenticatorTransport,
) *webauthn.Credential {
	credential := crossPlatformCredential(transports...)
	id := uuid.MustParse(aaguid)
	credential.Authenticator.AAGUID = id[:]
	return credential
}
