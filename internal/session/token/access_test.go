package token

import (
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/session/roles"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newTestAccessTokenService(t *testing.T, ttl time.Duration) *AccessTokenService {
	t.Helper()

	keySet, err := NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}

	return NewAccessTokenService(keySet, "authara-test", ttl)
}

func TestAccessTokenService_GenerateAndParse_AppAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	userID := uuid.New()
	sessionID := uuid.New()

	var rs roles.Roles
	rs.AddAdmin()
	rs.AddMonitor()

	tokenString, err := svc.Generate(userID, sessionID, AudienceApp, rs, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	claims, err := svc.Parse(tokenString, AudienceApp, now)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if claims.Subject != userID.String() {
		t.Fatalf("expected subject %q, got %q", userID.String(), claims.Subject)
	}

	if claims.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, claims.SessionID)
	}

	if claims.Issuer != "authara-test" {
		t.Fatalf("expected issuer %q, got %q", "authara-test", claims.Issuer)
	}

	if len(claims.Audience) != 1 || claims.Audience[0] != string(AudienceApp) {
		t.Fatalf("expected audience %q, got %v", AudienceApp, claims.Audience)
	}

	if len(claims.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(claims.Roles))
	}
}

func TestAccessTokenService_GenerateAndParse_AdminAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	userID := uuid.New()
	sessionID := uuid.New()

	var rs roles.Roles
	rs.AddAuditor()

	tokenString, err := svc.Generate(userID, sessionID, AudienceAdmin, rs, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	claims, err := svc.Parse(tokenString, AudienceAdmin, now)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(claims.Audience) != 1 || claims.Audience[0] != string(AudienceAdmin) {
		t.Fatalf("expected audience %q, got %v", AudienceAdmin, claims.Audience)
	}
}

func TestAccessTokenService_Parse_WrongAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	tokenString, err := svc.Generate(uuid.New(), uuid.New(), AudienceApp, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.Parse(tokenString, AudienceAdmin, now)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAccessTokenService_ParseAny_AllowsAppAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	tokenString, err := svc.Generate(uuid.New(), uuid.New(), AudienceApp, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ParseAny(tokenString, now)
	if err != nil {
		t.Fatalf("ParseAny failed: %v", err)
	}
}

func TestAccessTokenService_ParseAny_AllowsAdminAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	tokenString, err := svc.Generate(uuid.New(), uuid.New(), AudienceAdmin, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ParseAny(tokenString, now)
	if err != nil {
		t.Fatalf("ParseAny failed: %v", err)
	}
}

func TestAccessTokenService_Parse_ExpiredToken(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 1*time.Minute)

	tokenString, err := svc.Generate(uuid.New(), uuid.New(), AudienceApp, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.Parse(tokenString, AudienceApp, now.Add(2*time.Minute))
	if !errors.Is(err, ErrInvalidToken) && !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("expected ErrInvalidToken or ErrExpiredToken, got %v", err)
	}
}

func TestAccessTokenService_Parse_InvalidTokenString(t *testing.T) {
	svc := newTestAccessTokenService(t, 10*time.Minute)

	_, err := svc.Parse("not-a-jwt", AudienceApp, time.Now())
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAccessTokenService_Parse_UnknownKey(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	keySetA, err := NewKeySet("key-a", map[string][]byte{
		"key-a": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet A failed: %v", err)
	}

	keySetB, err := NewKeySet("key-b", map[string][]byte{
		"key-b": []byte("abcdefghijklmnopqrstuvwxyz123456"),
	})
	if err != nil {
		t.Fatalf("NewKeySet B failed: %v", err)
	}

	signer := NewAccessTokenService(keySetA, "authara-test", 10*time.Minute)
	verifier := NewAccessTokenService(keySetB, "authara-test", 10*time.Minute)

	tokenString, err := signer.Generate(uuid.New(), uuid.New(), AudienceApp, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = verifier.Parse(tokenString, AudienceApp, now)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAccessTokenService_Parse_WrongIssuer(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	keySet, err := NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}

	signer := NewAccessTokenService(keySet, "issuer-a", 10*time.Minute)
	verifier := NewAccessTokenService(keySet, "issuer-b", 10*time.Minute)

	tokenString, err := signer.Generate(uuid.New(), uuid.New(), AudienceApp, roles.Roles{}, now)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = verifier.Parse(tokenString, AudienceApp, now)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAccessTokenService_Parse_MissingSubjectInvalidClaims(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	kid, key := svc.keys.SigningKey()

	claims := AccessClaims{
		SessionID: uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    svc.issuer,
			Subject:   "",
			Audience:  jwt.ClaimStrings{string(AudienceApp)},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(svc.ttl)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = kid

	tokenString, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	_, err = svc.Parse(tokenString, AudienceApp, now)
	if !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("expected ErrInvalidClaims, got %v", err)
	}
}

func TestAccessTokenService_Parse_NilSessionIDInvalidClaims(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestAccessTokenService(t, 10*time.Minute)

	kid, key := svc.keys.SigningKey()

	claims := AccessClaims{
		SessionID: uuid.Nil,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    svc.issuer,
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{string(AudienceApp)},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(svc.ttl)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = kid

	tokenString, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	_, err = svc.Parse(tokenString, AudienceApp, now)
	if !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("expected ErrInvalidClaims, got %v", err)
	}
}

func TestValidateAccessClaims(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		claims *AccessClaims
		want   error
	}{
		{
			name: "valid claims",
			claims: &AccessClaims{
				SessionID: uuid.New(),
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   uuid.New().String(),
					ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
				},
			},
			want: nil,
		},
		{
			name: "missing expires at",
			claims: &AccessClaims{
				SessionID: uuid.New(),
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: uuid.New().String(),
				},
			},
			want: ErrExpiredToken,
		},
		{
			name: "expired token",
			claims: &AccessClaims{
				SessionID: uuid.New(),
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   uuid.New().String(),
					ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
				},
			},
			want: ErrExpiredToken,
		},
		{
			name: "empty subject",
			claims: &AccessClaims{
				SessionID: uuid.New(),
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   "",
					ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
				},
			},
			want: ErrInvalidClaims,
		},
		{
			name: "nil session id",
			claims: &AccessClaims{
				SessionID: uuid.Nil,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   uuid.New().String(),
					ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
				},
			},
			want: ErrInvalidClaims,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := validateAccessClaims(tt.claims, now)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestAccessTokenService_KeyFunc(t *testing.T) {
	svc := newTestAccessTokenService(t, 10*time.Minute)

	t.Run("missing kid", func(t *testing.T) {
		tok := &jwt.Token{
			Header: map[string]any{},
		}

		_, err := svc.keyFunc(tok)
		if !errors.Is(err, ErrUnknownKey) {
			t.Fatalf("expected ErrUnknownKey, got %v", err)
		}
	})

	t.Run("unknown kid", func(t *testing.T) {
		tok := &jwt.Token{
			Header: map[string]any{
				"kid": "does-not-exist",
			},
		}

		_, err := svc.keyFunc(tok)
		if !errors.Is(err, ErrUnknownKey) {
			t.Fatalf("expected ErrUnknownKey, got %v", err)
		}
	})

	t.Run("valid kid", func(t *testing.T) {
		tok := &jwt.Token{
			Header: map[string]any{
				"kid": "test-key",
			},
		}

		key, err := svc.keyFunc(tok)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if key == nil {
			t.Fatal("expected non-nil key")
		}
	})
}
