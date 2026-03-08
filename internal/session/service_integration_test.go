package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/domain"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/alexlup06-authgate/authgate/internal/session/token"
	"github.com/alexlup06-authgate/authgate/internal/testutil"
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
		"authgate-test",
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
