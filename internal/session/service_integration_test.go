package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newTestSessionService(t *testing.T, ttl time.Duration) *Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}

	accessTokens := token.NewAccessTokenService(
		keySet,
		"authara-test",
		ttl,
	)

	return New(SessionConfig{
		AccessTokens: accessTokens,
	})
}

func TestNew_DefaultsToNoopAccessPolicy(t *testing.T) {
	svc := New(SessionConfig{})

	if svc.accessPolicy == nil {
		t.Fatal("expected default access policy to be set")
	}

	allowed, err := svc.accessPolicy.IsEmailAllowed(context.Background(), "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error from default access policy: %v", err)
	}
	if !allowed {
		t.Fatal("expected default access policy to allow user")
	}
}

func TestNew_UsesProvidedAccessPolicy(t *testing.T) {
	custom := accesspolicy.NoopEmailAccessPolicy{}

	svc := New(SessionConfig{
		AccessPolicy: custom,
	})

	if svc.accessPolicy == nil {
		t.Fatal("expected provided access policy to be set")
	}
}

func TestCleanupExpiredDataDeletesWebAuthnChallenges(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
		svc := New(SessionConfig{Store: tdb.Store})

		expired, err := tdb.Store.CreateWebAuthnChallenge(ctx, domain.WebAuthnChallenge{
			Purpose:     domain.WebAuthnChallengePurposeAuthentication,
			Challenge:   "expired",
			SessionData: []byte(`{"challenge":"expired"}`),
			ExpiresAt:   now.Add(-time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateWebAuthnChallenge expired failed: %v", err)
		}

		consumedAt := now.Add(-time.Second)
		consumed, err := tdb.Store.CreateWebAuthnChallenge(ctx, domain.WebAuthnChallenge{
			Purpose:     domain.WebAuthnChallengePurposeAuthentication,
			Challenge:   "consumed",
			SessionData: []byte(`{"challenge":"consumed"}`),
			ExpiresAt:   now.Add(time.Minute),
			ConsumedAt:  &consumedAt,
		})
		if err != nil {
			t.Fatalf("CreateWebAuthnChallenge consumed failed: %v", err)
		}

		active, err := tdb.Store.CreateWebAuthnChallenge(ctx, domain.WebAuthnChallenge{
			Purpose:     domain.WebAuthnChallengePurposeAuthentication,
			Challenge:   "active",
			SessionData: []byte(`{"challenge":"active"}`),
			ExpiresAt:   now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("CreateWebAuthnChallenge active failed: %v", err)
		}

		if err := svc.CleanupExpiredData(ctx, now); err != nil {
			t.Fatalf("CleanupExpiredData failed: %v", err)
		}

		_, err = tdb.Store.GetWebAuthnChallengeByIDForUpdate(ctx, expired.ID)
		if !errors.Is(err, store.ErrWebAuthnChallengeNotFound) {
			t.Fatalf("expected expired challenge to be deleted, got %v", err)
		}
		_, err = tdb.Store.GetWebAuthnChallengeByIDForUpdate(ctx, consumed.ID)
		if !errors.Is(err, store.ErrWebAuthnChallengeNotFound) {
			t.Fatalf("expected consumed challenge to be deleted, got %v", err)
		}
		if _, err = tdb.Store.GetWebAuthnChallengeByIDForUpdate(ctx, active.ID); err != nil {
			t.Fatalf("expected active challenge to remain, got %v", err)
		}
	})
}

func TestValidateAccessToken_Succeeds(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestSessionService(t, 10*time.Minute)

	userID := uuid.New()
	sessionID := uuid.New()

	var rs roles.Roles
	rs.AddAdmin()
	rs.AddMonitor()

	accessToken, err := svc.accessTokens.Generate(
		userID,
		sessionID,
		token.AudienceAdmin,
		rs,
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	identity, err := svc.ValidateAccessToken(
		accessToken,
		token.AudienceAdmin,
		now,
	)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if identity.UserID != userID {
		t.Fatalf("expected user id %q, got %q", userID, identity.UserID)
	}
	if identity.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, identity.SessionID)
	}
	if !identity.Roles.IsAdmin() {
		t.Fatal("expected admin role to be present")
	}
	if !identity.Roles.IsMonitor() {
		t.Fatal("expected monitor role to be present")
	}
}

func TestValidateAccessToken_WrongAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestSessionService(t, 10*time.Minute)

	accessToken, err := svc.accessTokens.Generate(
		uuid.New(),
		uuid.New(),
		token.AudienceApp,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAccessToken(
		accessToken,
		token.AudienceAdmin,
		now,
	)
	if !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateAccessToken_InvalidToken(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	_, err := svc.ValidateAccessToken(
		"not-a-token",
		token.AudienceApp,
		time.Now(),
	)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateAnyAccessToken_AcceptsAppAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestSessionService(t, 10*time.Minute)

	accessToken, err := svc.accessTokens.Generate(
		uuid.New(),
		uuid.New(),
		token.AudienceApp,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAnyAccessToken(accessToken, now)
	if err != nil {
		t.Fatalf("ValidateAnyAccessToken failed: %v", err)
	}
}

func TestValidateAnyAccessToken_AcceptsAdminAudience(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestSessionService(t, 10*time.Minute)

	accessToken, err := svc.accessTokens.Generate(
		uuid.New(),
		uuid.New(),
		token.AudienceAdmin,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAnyAccessToken(accessToken, now)
	if err != nil {
		t.Fatalf("ValidateAnyAccessToken failed: %v", err)
	}
}

func TestIdentityFromClaims_InvalidSubject(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	claims := &token.AccessClaims{
		SessionID: uuid.New(),
		Roles:     []roles.Role{roles.AutharaAdmin},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "not-a-uuid",
		},
	}
	// easier and compile-safe: assign on embedded RegisteredClaims after construction
	claims.Subject = "not-a-uuid"

	_, err := svc.identityFromClaims(claims)
	if !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestIdentityFromClaims_NilUUIDSubject(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	claims := &token.AccessClaims{
		SessionID: uuid.New(),
		Roles:     []roles.Role{roles.AutharaAdmin},
	}
	claims.Subject = uuid.Nil.String()

	_, err := svc.identityFromClaims(claims)
	if !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestIdentityFromClaims_InvalidRoles(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	claims := &token.AccessClaims{
		SessionID: uuid.New(),
		Roles:     []roles.Role{"authara:unknown"},
	}
	claims.Subject = uuid.New().String()

	_, err := svc.identityFromClaims(claims)
	if err == nil {
		t.Fatal("expected error for invalid roles")
	}
}

func TestIdentityFromClaims_Succeeds(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	userID := uuid.New()
	sessionID := uuid.New()

	claims := &token.AccessClaims{
		SessionID: sessionID,
		Roles:     []roles.Role{roles.AutharaAdmin, roles.AutharaAuditor},
	}
	claims.Subject = userID.String()

	identity, err := svc.identityFromClaims(claims)
	if err != nil {
		t.Fatalf("identityFromClaims failed: %v", err)
	}

	if identity.UserID != userID {
		t.Fatalf("expected user id %q, got %q", userID, identity.UserID)
	}
	if identity.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, identity.SessionID)
	}
	if !identity.Roles.IsAdmin() {
		t.Fatal("expected admin role")
	}
	if !identity.Roles.IsAuditor() {
		t.Fatal("expected auditor role")
	}
	if identity.Roles.IsMonitor() {
		t.Fatal("did not expect monitor role")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	tokenA, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken failed: %v", err)
	}
	if tokenA == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if strings.Contains(tokenA, "=") {
		t.Fatal("expected raw URL encoding without padding")
	}

	tokenB, err := generateRefreshToken()
	if err != nil {
		t.Fatalf("generateRefreshToken failed: %v", err)
	}
	if tokenB == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if tokenA == tokenB {
		t.Fatal("expected generated refresh tokens to differ")
	}
}

func TestHashRefreshToken(t *testing.T) {
	input := "refresh-token"

	got1 := hashRefreshToken(input)
	got2 := hashRefreshToken(input)
	got3 := hashRefreshToken("different-token")

	if got1 != got2 {
		t.Fatal("expected hash to be deterministic")
	}
	if got1 == got3 {
		t.Fatal("expected different inputs to produce different hashes")
	}
	if len(got1) != 64 {
		t.Fatalf("expected SHA-256 hex length 64, got %d", len(got1))
	}
}

func TestShouldRotate(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		rt       domain.RefreshToken
		rotation time.Duration
		want     bool
	}{
		{
			name: "negative rotation always rotates",
			rt: domain.RefreshToken{
				CreatedAt: now,
			},
			rotation: -1,
			want:     true,
		},
		{
			name: "zero rotation never rotates",
			rt: domain.RefreshToken{
				CreatedAt: now.Add(-10 * time.Minute),
			},
			rotation: 0,
			want:     false,
		},
		{
			name: "below threshold does not rotate",
			rt: domain.RefreshToken{
				CreatedAt: now.Add(-4 * time.Minute),
			},
			rotation: 5 * time.Minute,
			want:     false,
		},
		{
			name: "exact threshold rotates",
			rt: domain.RefreshToken{
				CreatedAt: now.Add(-5 * time.Minute),
			},
			rotation: 5 * time.Minute,
			want:     true,
		},
		{
			name: "above threshold rotates",
			rt: domain.RefreshToken{
				CreatedAt: now.Add(-6 * time.Minute),
			},
			rotation: 5 * time.Minute,
			want:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRotate(tt.rt, now, tt.rotation)
			if got != tt.want {
				t.Fatalf("shouldRotate(...) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanAccessAudience(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*roles.Roles)
		audience token.Audience
		want     bool
	}{
		{
			name:     "app audience allows everyone",
			setup:    func(r *roles.Roles) {},
			audience: token.AudienceApp,
			want:     true,
		},
		{
			name:     "admin audience denies empty roles",
			setup:    func(r *roles.Roles) {},
			audience: token.AudienceAdmin,
			want:     false,
		},
		{
			name: "admin audience allows admin",
			setup: func(r *roles.Roles) {
				r.AddAdmin()
			},
			audience: token.AudienceAdmin,
			want:     true,
		},
		{
			name: "admin audience allows auditor",
			setup: func(r *roles.Roles) {
				r.AddAuditor()
			},
			audience: token.AudienceAdmin,
			want:     true,
		},
		{
			name: "admin audience allows monitor",
			setup: func(r *roles.Roles) {
				r.AddMonitor()
			},
			audience: token.AudienceAdmin,
			want:     true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var rs roles.Roles
			tt.setup(&rs)

			got := canAccessAudience(rs, tt.audience)
			if got != tt.want {
				t.Fatalf("canAccessAudience(...) = %v, want %v", got, tt.want)
			}
		})
	}
}
