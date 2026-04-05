package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestRefreshSession_ReuseDetection_WhenRotationAlways(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

		svc := newTestSessionService(t, tdb, 24*time.Hour, 24*time.Hour, -1)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "reuse@example.com",
			Username: "reuse-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		_, refresh1, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			"test-agent",
			now,
		)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		_, refresh2, err := svc.RefreshSession(
			ctx,
			refresh1,
			token.AudienceApp,
			now.Add(1*time.Minute),
		)
		if err != nil {
			t.Fatalf("first RefreshSession failed: %v", err)
		}

		if refresh2 == "" {
			t.Fatal("expected rotated refresh token, got empty string")
		}
		if refresh2 == refresh1 {
			t.Fatal("expected rotated refresh token to differ from original")
		}

		_, _, err = svc.RefreshSession(
			ctx,
			refresh1,
			token.AudienceApp,
			now.Add(2*time.Minute),
		)
		if !errors.Is(err, session.ErrRefreshTokenReuse) {
			t.Fatalf("expected ErrRefreshTokenReuse, got: %v", err)
		}
	})
}

func TestRefreshSession_ExpiredRefreshToken(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		svc := newTestSessionService(t, tdb, 24*time.Hour, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "expired@example.com",
			Username: "expired-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.RefreshSession(
			ctx,
			refresh,
			token.AudienceApp,
			now.Add(30*24*time.Hour), // far in future
		)

		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})
}

func TestRefreshSession_NoRotation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		svc := newTestSessionService(t, tdb, 24*time.Hour, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "norotation@example.com",
			Username: "norotation-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh1, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		_, refresh2, err := svc.RefreshSession(
			ctx,
			refresh1,
			token.AudienceApp,
			now.Add(1*time.Minute),
		)
		if err != nil {
			t.Fatal(err)
		}

		if refresh1 != refresh2 {
			t.Fatal("expected refresh token to stay the same when rotation disabled")
		}
	})
}

func TestRefreshSession_SessionExpired(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		svc := newTestSessionService(t, tdb, 24*time.Hour, 1*time.Minute, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "sessionexpired@example.com",
			Username: "sessionexpired-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.RefreshSession(
			ctx,
			refresh,
			token.AudienceApp,
			now.Add(2*time.Minute),
		)

		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})
}

func TestCreateSession_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "create-session@example.com",
			Username: "create-session-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		accessToken, refreshToken, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			"test-agent",
			now,
		)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		if accessToken == "" {
			t.Fatal("expected non-empty access token")
		}
		if refreshToken == "" {
			t.Fatal("expected non-empty refresh token")
		}
	})
}

func TestCreateSession_ForbiddenForAdminAudienceWhenNotAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "not-admin@example.com",
			Username: "not-admin-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceAdmin,
			"test-agent",
			now,
		)
		if !errors.Is(err, session.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestCreateSession_AllowedForAdminAudienceWhenAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "admin@example.com",
			Username: "admin-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := tdb.Store.AddUserPlatformRoleByName(ctx, user.ID, "admin"); err != nil {
			t.Fatal(err)
		}

		accessToken, refreshToken, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceAdmin,
			"test-agent",
			now,
		)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if accessToken == "" || refreshToken == "" {
			t.Fatal("expected non-empty tokens")
		}
	})
}

func TestCreateSession_UserDisabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "disabled-create@example.com",
			Username: "disabled-create-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := tdb.Store.DisableUser(ctx, user.ID, now); err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			"test-agent",
			now,
		)
		if !errors.Is(err, session.ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}
	})
}

func TestCreateSession_UserNotAllowed(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		svc := newTestSessionServiceWithPolicy(
			t,
			tdb,
			10*time.Minute,
			24*time.Hour,
			0,
			denyAllPolicy{},
		)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "denied@example.com",
			Username: "denied-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			"test-agent",
			now,
		)
		if !errors.Is(err, session.ErrUserNotAllowed) {
			t.Fatalf("expected ErrUserNotAllowed, got %v", err)
		}
	})
}

func TestRefreshSession_UserDisabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "disabled-refresh@example.com",
			Username: "disabled-refresh-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		if err := tdb.Store.DisableUser(ctx, user.ID, now.Add(1*time.Minute)); err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh, token.AudienceApp, now.Add(2*time.Minute))
		if !errors.Is(err, session.ErrUserDisabled) {
			t.Fatalf("expected ErrUserDisabled, got %v", err)
		}
	})
}

func TestRefreshSession_UserNotAllowed(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()

		allowSvc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "refresh-denied@example.com",
			Username: "refresh-denied-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := allowSvc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		denySvc := newTestSessionServiceWithPolicy(
			t,
			tdb,
			10*time.Minute,
			24*time.Hour,
			0,
			denyAllPolicy{},
		)

		_, _, err = denySvc.RefreshSession(ctx, refresh, token.AudienceApp, now.Add(1*time.Minute))
		if !errors.Is(err, session.ErrUserNotAllowed) {
			t.Fatalf("expected ErrUserNotAllowed, got %v", err)
		}
	})
}

func TestRefreshSession_AdminAudienceForbiddenWhenNotAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "refresh-app-only@example.com",
			Username: "refresh-app-only-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh, token.AudienceAdmin, now.Add(1*time.Minute))
		if !errors.Is(err, session.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})
}

func TestCleanupExpiredData(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 1*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "cleanup@example.com",
			Username: "cleanup-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh, token.AudienceApp, now.Add(2*time.Hour))
		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken before cleanup, got %v", err)
		}

		if err := svc.CleanupExpiredData(ctx, now.Add(3*time.Hour)); err != nil {
			t.Fatalf("CleanupExpiredData failed: %v", err)
		}
	})
}

func TestLogout_MissingRefreshTokenSucceeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		err := svc.Logout(ctx, "not-a-real-token")
		if err != nil {
			t.Fatalf("expected logout to succeed for missing token, got %v", err)
		}
	})
}

func TestLogout_RevokesSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "logout@example.com",
			Username: "logout-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		if err := svc.Logout(ctx, refresh); err != nil {
			t.Fatalf("Logout failed: %v", err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh, token.AudienceApp, now.Add(1*time.Minute))
		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken after logout, got %v", err)
		}
	})
}

func TestRevokeAllSessions(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "revoke-all@example.com",
			Username: "revoke-all-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		_, refresh1, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent-1", now)
		if err != nil {
			t.Fatal(err)
		}
		_, refresh2, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent-2", now)
		if err != nil {
			t.Fatal(err)
		}

		if err := svc.RevokeAllSessions(ctx, user.ID); err != nil {
			t.Fatalf("RevokeAllSessions failed: %v", err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh1, token.AudienceApp, now.Add(1*time.Minute))
		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken for refresh1, got %v", err)
		}

		_, _, err = svc.RefreshSession(ctx, refresh2, token.AudienceApp, now.Add(1*time.Minute))
		if !errors.Is(err, session.ErrInvalidRefreshToken) {
			t.Fatalf("expected ErrInvalidRefreshToken for refresh2, got %v", err)
		}
	})
}

func TestValidateAccessToken_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "validate@example.com",
			Username: "validate-user",
		})
		if err != nil {
			t.Fatal(err)
		}

		accessToken, _, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, "agent", now)
		if err != nil {
			t.Fatal(err)
		}

		identity, err := svc.ValidateAccessToken(ctx, accessToken, now)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}
		if identity.UserID != user.ID {
			t.Fatalf("expected user id %q, got %q", user.ID, identity.UserID)
		}
		if identity.SessionID == uuid.Nil {
			t.Fatal("expected non-nil session id")
		}
	})
}

func TestValidateAccessToken_InvalidToken(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now()
		svc := newTestSessionService(t, tdb, 10*time.Minute, 24*time.Hour, 0)

		_, err := svc.ValidateAccessToken(ctx, "not-a-token", now)
		if err == nil {
			t.Fatal("expected invalid token error")
		}
	})
}

type denyAllPolicy struct{}

func (denyAllPolicy) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func newTestSessionServiceWithPolicy(
	t *testing.T,
	tdb *testutil.TestDB,
	accessTTL time.Duration,
	sessionTTL time.Duration,
	rotation time.Duration,
	policy accesspolicy.EmailAccessPolicy,
) *session.Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("token.NewKeySet failed: %v", err)
	}

	accessTokens := token.NewAccessTokenService(
		keySet,
		"authara-test",
		accessTTL,
	)

	return session.New(session.SessionConfig{
		Store:                tdb.Store,
		Tx:                   tdb.Tx,
		AccessTokens:         accessTokens,
		SessionTTL:           sessionTTL,
		RefreshTokenTTL:      7 * 24 * time.Hour,
		RefreshTokenRotation: rotation,
		AccessPolicy:         policy,
	})
}

func newTestSessionService(
	t *testing.T,
	tdb *testutil.TestDB,
	accessTTL time.Duration,
	sessionTTL time.Duration,
	rotation time.Duration,
) *session.Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("token.NewKeySet failed: %v", err)
	}

	accessTokens := token.NewAccessTokenService(
		keySet,
		"authara-test",
		accessTTL,
	)

	return session.New(session.SessionConfig{
		Store:                tdb.Store,
		Tx:                   tdb.Tx,
		AccessTokens:         accessTokens,
		SessionTTL:           sessionTTL,
		RefreshTokenTTL:      7 * 24 * time.Hour,
		RefreshTokenRotation: rotation,
	})
}
