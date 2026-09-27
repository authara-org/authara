package session

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

func newDBSessionService(t *testing.T, tdb *testutil.TestDB, ttl time.Duration) *Service {
	return newDBSessionServiceWithCache(t, tdb, ttl, nil, 0)
}

func newDBSessionServiceWithCache(
	t *testing.T,
	tdb *testutil.TestDB,
	ttl time.Duration,
	cacheStore cache.Cache,
	refreshTokenRotation time.Duration,
) *Service {
	return newDBSessionServiceWithPolicy(
		t,
		tdb,
		ttl,
		cacheStore,
		time.Hour,
		time.Hour,
		refreshTokenRotation,
	)
}

func newDBSessionServiceWithPolicy(
	t *testing.T,
	tdb *testutil.TestDB,
	accessTokenTTL time.Duration,
	cacheStore cache.Cache,
	sessionTTL time.Duration,
	refreshTokenTTL time.Duration,
	refreshTokenRotation time.Duration,
) *Service {
	t.Helper()

	keySet, err := token.NewKeySet("test-key", map[string][]byte{
		"test-key": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatalf("NewKeySet failed: %v", err)
	}

	var revocations *token.AccessTokenRevocations
	if cacheStore != nil {
		revocations = token.NewAccessTokenRevocations(cacheStore, accessTokenTTL)
	}

	return New(SessionConfig{
		Store: tdb.Store,
		Tx:    tdb.Tx,
		AccessTokens: token.NewAccessTokenService(
			keySet,
			"authara-test",
			accessTokenTTL,
		),
		AccessTokenRevocations: revocations,
		SessionTTL:             sessionTTL,
		RefreshTokenTTL:        refreshTokenTTL,
		RefreshTokenRotation:   refreshTokenRotation,
		Organizations:          organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}),
		SecurityEvents:         securityevent.NewStandard(tdb.Store, 180*24*time.Hour),
	})
}

var errSessionTestCache = errors.New("session test cache unavailable")

func TestCreatePasskeySessionWaitsForRestrictionAndFailsClosed(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email: "passkey-session-race-" + uuid.NewString() + "@example.com", Username: "passkey-session-race-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tdb.Store.DeleteUser(context.Background(), user.ID) })
	passkey, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
		UserID: user.ID, CredentialID: []byte("passkey-session-race-" + uuid.NewString()), PublicKey: []byte("public-key"),
	})
	if err != nil {
		t.Fatal(err)
	}

	txCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if err := tdb.Store.LockUserForUpdate(txCtx, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := tdb.Store.RestrictPasskey(txCtx, passkey.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	svc := newDBSessionService(t, tdb, time.Minute)
	done := make(chan error, 1)
	go func() {
		_, _, err := svc.CreatePasskeySession(
			ctx, user.ID, passkey.ID, token.AudienceApp, "test-agent", time.Now().UTC(), "",
		)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("passkey session completed before restriction committed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tdb.Tx.Commit(txCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrAuthenticationMethodUnavailable) {
			t.Fatalf("expected unavailable passkey, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("passkey session did not finish after restriction commit")
	}
}

func TestRecentAuthenticationPolicy(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "recent-auth-" + uuid.NewString() + "@example.com",
			Username: "recent-auth-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}
		svc := newDBSessionService(t, tdb, 10*time.Minute)
		accessToken, refreshToken, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			domain.AuthenticationMethodPassword,
			"recent-auth-test",
			now,
			"",
		)
		if err != nil {
			t.Fatal(err)
		}
		identity, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatal(err)
		}
		events, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:      domain.SecurityEventAuthenticationLogin,
			Outcome:   domain.SecurityEventOutcomeSuccess,
			SessionID: &identity.SessionID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].AuthenticationMethod != domain.AuthenticationMethodPassword || events[0].UserID == nil || *events[0].UserID != user.ID {
			t.Fatalf("unexpected authentication event: %+v", events)
		}

		if err := svc.RequireRecentAuthentication(ctx, user.ID, identity.SessionID, now.Add(10*time.Minute)); err != nil {
			t.Fatalf("boundary should be fresh: %v", err)
		}
		if err := svc.RequireRecentAuthentication(ctx, user.ID, identity.SessionID, now.Add(10*time.Minute+time.Nanosecond)); !errors.Is(err, ErrRecentAuthenticationRequired) {
			t.Fatalf("expected stale session, got %v", err)
		}
		disabled := New(SessionConfig{
			Store: tdb.Store,
			Policy: config.SessionPolicyReaderFunc(func() config.SessionPolicy {
				return config.SessionPolicy{
					RecentAuthenticationEnabled: false,
					RecentAuthenticationWindow:  10 * time.Minute,
				}
			}),
		})
		if err := disabled.RequireRecentAuthentication(ctx, user.ID, identity.SessionID, now.Add(11*time.Minute)); err != nil {
			t.Fatalf("disabled policy rejected an active stale session: %v", err)
		}
		if err := disabled.RequireRecentAuthentication(ctx, uuid.New(), identity.SessionID, now.Add(11*time.Minute)); !errors.Is(err, ErrInvalidSession) {
			t.Fatalf("disabled policy accepted another user's session: %v", err)
		}

		markedAt := now.Add(11 * time.Minute)
		if err := svc.MarkRecentlyAuthenticated(ctx, user.ID, identity.SessionID, domain.AuthenticationMethodPasskey, markedAt); err != nil {
			t.Fatal(err)
		}
		if err := svc.RequireRecentAuthentication(ctx, user.ID, identity.SessionID, markedAt); err != nil {
			t.Fatalf("fresh proof rejected: %v", err)
		}
		stored, err := tdb.Store.GetSessionByID(ctx, identity.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.AuthenticatedAt == nil || !stored.AuthenticatedAt.Equal(markedAt) || stored.AuthenticationMethod != domain.AuthenticationMethodPasskey {
			t.Fatalf("unexpected authentication provenance: %#v", stored)
		}

		if _, _, err := svc.RefreshSession(ctx, refreshToken, token.AudienceApp, markedAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		afterRefresh, err := tdb.Store.GetSessionByID(ctx, identity.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		if afterRefresh.AuthenticatedAt == nil || !afterRefresh.AuthenticatedAt.Equal(markedAt) {
			t.Fatal("refresh changed authentication freshness")
		}

		legacy, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: identity.OrganizationID,
			ExpiresAt:            now.Add(time.Hour),
			UserAgent:            "legacy",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.RequireRecentAuthentication(ctx, user.ID, legacy.ID, now); !errors.Is(err, ErrRecentAuthenticationRequired) {
			t.Fatalf("legacy session should fail stale, got %v", err)
		}
		if err := svc.MarkRecentlyAuthenticated(ctx, uuid.New(), legacy.ID, domain.AuthenticationMethodPassword, now); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("proof must not update another user's session, got %v", err)
		}
		unchanged, err := tdb.Store.GetSessionByID(ctx, legacy.ID)
		if err != nil {
			t.Fatal(err)
		}
		if unchanged.AuthenticatedAt != nil || unchanged.AuthenticationMethod != "" {
			t.Fatalf("wrong-user proof changed session provenance: %#v", unchanged)
		}
	})
}

func TestAuthenticationChallengeIsSessionBoundSingleUseAndExpires(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "authentication-challenge-" + uuid.NewString() + "@example.com",
			Username: "authentication-challenge-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		row, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: now.Add(time.Hour), UserAgent: "challenge-test",
		})
		if err != nil {
			t.Fatal(err)
		}
		svc := newDBSessionService(t, tdb, 10*time.Minute)

		challenge, err := svc.StartAuthenticationChallenge(ctx, user.ID, row.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		if !challenge.ExpiresAt.Equal(now.Add(authenticationChallengeTTL)) {
			t.Fatalf("challenge expiry = %s", challenge.ExpiresAt)
		}
		reused, err := svc.StartAuthenticationChallenge(ctx, user.ID, row.ID, now.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if reused.ID != challenge.ID || !reused.ExpiresAt.Equal(challenge.ExpiresAt) {
			t.Fatalf("active challenge was replaced: first=%+v reused=%+v", challenge, reused)
		}
		if err := svc.ValidateAuthenticationChallenge(ctx, uuid.New(), row.ID, challenge.ID, now); !errors.Is(err, ErrAuthenticationChallengeInvalid) {
			t.Fatalf("challenge accepted another user: %v", err)
		}
		if err := svc.ValidateCompletedAuthenticationChallenge(ctx, user.ID, row.ID, challenge.ID); !errors.Is(err, ErrAuthenticationChallengeInvalid) {
			t.Fatalf("incomplete challenge accepted as complete: %v", err)
		}
		if err := svc.CompleteAuthenticationChallenge(ctx, user.ID, row.ID, challenge.ID, domain.AuthenticationMethodPasskey, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if err := svc.ValidateAuthenticationChallenge(ctx, user.ID, row.ID, challenge.ID, now.Add(time.Minute)); !errors.Is(err, ErrAuthenticationChallengeInvalid) {
			t.Fatalf("consumed challenge remained valid: %v", err)
		}
		if err := svc.ValidateCompletedAuthenticationChallenge(ctx, user.ID, row.ID, challenge.ID); err != nil {
			t.Fatalf("completed challenge was not recognized: %v", err)
		}
		if err := svc.RequireRecentAuthentication(ctx, user.ID, row.ID, now.Add(time.Minute)); err != nil {
			t.Fatalf("completed challenge did not refresh session: %v", err)
		}
		events, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:      domain.SecurityEventAuthenticationReauthenticated,
			Outcome:   domain.SecurityEventOutcomeSuccess,
			SessionID: &row.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 || events[0].AuthenticationMethod != domain.AuthenticationMethodPasskey {
			t.Fatalf("unexpected reauthentication event: %+v", events)
		}

		expiring, err := svc.StartAuthenticationChallenge(ctx, user.ID, row.ID, now.Add(2*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.CompleteAuthenticationChallenge(ctx, user.ID, row.ID, expiring.ID, domain.AuthenticationMethodPassword, expiring.ExpiresAt); !errors.Is(err, ErrAuthenticationChallengeInvalid) {
			t.Fatalf("expired challenge completion = %v", err)
		}
		events, err = tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:      domain.SecurityEventAuthenticationReauthenticated,
			SessionID: &row.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 1 {
			t.Fatalf("failed completion created a success event: %+v", events)
		}
	})
}

type sessionTestCache struct {
	values map[string][]byte
	err    error
}

type blockingSetMaxCache struct {
	cache.Cache
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingSetMaxCache) SetMaxInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
		return c.Cache.SetMaxInt64(ctx, key, value, ttl)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *sessionTestCache) Get(_ context.Context, key string) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	value, ok := c.values[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return value, nil
}

func (c *sessionTestCache) GetMany(_ context.Context, keys ...string) ([][]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	values := make([][]byte, len(keys))
	for i, key := range keys {
		values[i] = c.values[key]
	}
	return values, nil
}

func (c *sessionTestCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	if c.err != nil {
		return c.err
	}
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *sessionTestCache) SetMaxInt64(_ context.Context, key string, value int64, _ time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if current, ok := c.values[key]; ok {
		currentValue, err := strconv.ParseInt(string(current), 10, 64)
		if err != nil {
			return err
		}
		if currentValue >= value {
			return nil
		}
	}
	c.values[key] = []byte(strconv.FormatInt(value, 10))
	return nil
}

func (c *sessionTestCache) Delete(_ context.Context, key string) error {
	if c.err != nil {
		return c.err
	}
	delete(c.values, key)
	return nil
}

func (c *sessionTestCache) Close() error { return nil }

func TestNew_DefaultsToNoopAccessPolicy(t *testing.T) {
	svc := New(SessionConfig{})

	if svc.accessPolicy == nil {
		t.Fatal("expected default access policy to be set")
	}
	if svc.organizations != nil {
		t.Fatal("expected organization service to require explicit configuration")
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

func TestCleanupExpiredDataRetainsConsumedRefreshTokensUntilFamilyEnds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "refresh-retention@example.com",
			Username: "refresh-retention",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		svc := newDBSessionServiceWithPolicy(
			t,
			tdb,
			10*time.Minute,
			cache.NewNoop(),
			2*time.Hour,
			30*time.Minute,
			-time.Nanosecond,
		)
		_, originalRefreshToken, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			domain.AuthenticationMethodPassword,
			"retention-test",
			now,
			"",
		)
		if err != nil {
			t.Fatal(err)
		}

		rotatedAt := now.Add(20 * time.Minute)
		_, descendantRefreshToken, err := svc.RefreshSession(
			ctx,
			originalRefreshToken,
			token.AudienceApp,
			rotatedAt,
		)
		if err != nil {
			t.Fatal(err)
		}
		original, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(originalRefreshToken))
		if err != nil {
			t.Fatal(err)
		}
		if original.ConsumedAt == nil {
			t.Fatal("expected original refresh token to be consumed")
		}

		expiredUnconsumedHash := hashRefreshToken("expired-unconsumed-" + uuid.NewString())
		if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
			SessionID:      original.SessionID,
			OrganizationID: original.OrganizationID,
			TokenHash:      expiredUnconsumedHash,
			CreatedAt:      now,
			ExpiresAt:      now.Add(30 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}

		cleanupAt := now.Add(40 * time.Minute)
		if err := svc.CleanupExpiredData(ctx, cleanupAt); err != nil {
			t.Fatal(err)
		}
		retained, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(originalRefreshToken))
		if err != nil {
			t.Fatalf("consumed refresh-token tombstone was removed before family expiry: %v", err)
		}
		if retained.ConsumedAt == nil {
			t.Fatal("retained refresh token is not marked consumed")
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, expiredUnconsumedHash); !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("expired unconsumed refresh token survived cleanup: %v", err)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(descendantRefreshToken)); err != nil {
			t.Fatalf("active descendant refresh token was removed: %v", err)
		}

		reusedAt := cleanupAt.Add(time.Minute)
		_, _, err = svc.RefreshSession(ctx, originalRefreshToken, token.AudienceApp, reusedAt)
		if !errors.Is(err, ErrRefreshTokenReuse) {
			t.Fatalf("expected retained tombstone replay to return ErrRefreshTokenReuse, got %v", err)
		}
		persistedSession, err := tdb.Store.GetSessionByID(ctx, original.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		if persistedSession.RevokedAt == nil || !persistedSession.RevokedAt.Equal(reusedAt) {
			t.Fatalf("session revoked_at = %v, want %v", persistedSession.RevokedAt, reusedAt)
		}
		if _, _, err := svc.RefreshSession(
			ctx,
			descendantRefreshToken,
			token.AudienceApp,
			reusedAt.Add(time.Minute),
		); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("expected descendant refresh token to be rejected, got %v", err)
		}

		if err := svc.CleanupExpiredData(ctx, reusedAt.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(originalRefreshToken)); !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("consumed tombstone survived terminal family cleanup: %v", err)
		}
	})
}

func TestCleanupExpiredDataRemovesConsumedRefreshTokensAfterSessionExpiry(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "expired-refresh-family@example.com",
			Username: "expired-refresh-family",
		})
		if err != nil {
			t.Fatal(err)
		}
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: organization.ID,
			ExpiresAt:            now.Add(-time.Minute),
			UserAgent:            "retention-expiry-test",
		})
		if err != nil {
			t.Fatal(err)
		}

		consumedAt := now.Add(-time.Hour)
		refreshTokenHash := hashRefreshToken("expired-family-" + uuid.NewString())
		if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
			SessionID:      session.ID,
			OrganizationID: organization.ID,
			TokenHash:      refreshTokenHash,
			CreatedAt:      now.Add(-2 * time.Hour),
			ExpiresAt:      now.Add(-90 * time.Minute),
			ConsumedAt:     &consumedAt,
		}); err != nil {
			t.Fatal(err)
		}

		svc := New(SessionConfig{Store: tdb.Store})
		if err := svc.CleanupExpiredData(ctx, now); err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.GetSessionByID(ctx, session.ID); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("expired refresh-token family session survived cleanup: %v", err)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, refreshTokenHash); !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("consumed tombstone survived session-family expiry: %v", err)
		}
	})
}

func TestCreateSessionAddsOrganizationContext(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "session-org@example.com",
			Username: "session-org",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		svc := newDBSessionService(t, tdb, 10*time.Minute)
		accessToken, refreshToken, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "test-agent", now, "203.0.113.42")
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		identity, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}
		if identity.OrganizationID == uuid.Nil {
			t.Fatal("expected organization id in access token")
		}
		if identity.OrganizationRole != domain.OrganizationRoleOwner {
			t.Fatalf("expected owner role, got %q", identity.OrganizationRole)
		}

		session, err := tdb.Store.GetSessionByID(ctx, identity.SessionID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if session.ActiveOrganizationID != identity.OrganizationID {
			t.Fatalf("expected session org %q, got %q", identity.OrganizationID, session.ActiveOrganizationID)
		}

		rt, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
		if err != nil {
			t.Fatalf("GetRefreshTokenByHash failed: %v", err)
		}
		if rt.OrganizationID != identity.OrganizationID {
			t.Fatalf("expected refresh org %q, got %q", identity.OrganizationID, rt.OrganizationID)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateNewSignIn); got != 1 {
			t.Fatalf("new sign-in email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateNewSignIn)
		if data[email.TemplateVariableIPAddress] != "203.0.113.42" ||
			data[email.TemplateVariableUserAgent] != "test-agent" ||
			data[email.TemplateVariableOccurredAt] != "2026-05-14T12:00:00Z" {
			t.Fatalf("unexpected new sign-in template data: %#v", data)
		}
	})
}

func TestLogoutCancelsPendingEmailChange(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "logout-email-change@example.com",
			Username: "logout-email-change",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		svc := newDBSessionService(t, tdb, 10*time.Minute)
		accessToken, refreshToken, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "logout-test", now, "")
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		refresh, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
		if err != nil {
			t.Fatalf("GetRefreshTokenByHash failed: %v", err)
		}
		challengeRow, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:     domain.ChallengePurposeEmailChange,
			Email:       "logout-email-change-new@example.com",
			ExpiresAt:   now.Add(time.Hour),
			MaxAttempts: 5,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		if _, err := tdb.Store.CreatePendingEmailChange(ctx, domain.PendingEmailChange{
			ChallengeID:         challengeRow.ID,
			UserID:              user.ID,
			InitiatingSessionID: refresh.SessionID,
			OldEmail:            user.Email,
			NewEmail:            "logout-email-change-new@example.com",
		}); err != nil {
			t.Fatalf("CreatePendingEmailChange failed: %v", err)
		}

		if err := svc.Logout(ctx, refreshToken, accessToken); err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
		if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeRow.ID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
			t.Fatalf("pending email change survived logout: %v", err)
		}
	})
}

func TestLogoutRevocationStoreFailureDoesNotCommitDatabaseOnlyRevocation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "logout-revocation-failure-" + uuid.NewString() + "@example.com",
			Username: "logout-revocation-failure-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		cacheStore := &sessionTestCache{values: map[string][]byte{}}
		svc := newDBSessionServiceWithCache(t, tdb, 10*time.Minute, cacheStore, 0)
		accessToken, refreshToken, err := svc.CreateSession(
			ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "logout-test", now, "",
		)
		if err != nil {
			t.Fatal(err)
		}
		identity, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatal(err)
		}

		cacheStore.err = errSessionTestCache
		err = svc.Logout(ctx, refreshToken, accessToken)
		if !errors.Is(err, token.ErrRevocationStoreUnavailable) || !errors.Is(err, errSessionTestCache) {
			t.Fatalf("Logout error = %v", err)
		}
		persisted, err := tdb.Store.GetSessionByID(ctx, identity.SessionID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.RevokedAt != nil {
			t.Fatalf("session was revoked despite failed marker write: %v", persisted.RevokedAt)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken)); err != nil {
			t.Fatalf("refresh token was removed despite failed marker write: %v", err)
		}

		cacheStore.err = nil
		if err := svc.Logout(ctx, refreshToken, accessToken); err != nil {
			t.Fatalf("retry Logout failed: %v", err)
		}
		if _, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now); !errors.Is(err, token.ErrRevokedToken) {
			t.Fatalf("access token after successful retry = %v, want ErrRevokedToken", err)
		}
	})
}

func TestLogoutSerializesConcurrentRefreshBeforePublishingRevocation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suffix := uuid.NewString()
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "logout-refresh-race-" + suffix + "@example.com",
		Username: "logout-refresh-race-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
	})

	cacheStore := &blockingSetMaxCache{
		Cache:   &sessionTestCache{values: map[string][]byte{}},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	released := false
	defer func() {
		if !released {
			close(cacheStore.release)
		}
	}()
	svc := newDBSessionServiceWithCache(t, tdb, 10*time.Minute, cacheStore, 0)
	issuedAt := time.Now().UTC().Add(-time.Minute)
	_, refreshToken, err := svc.CreateSession(
		ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "logout-refresh-race", issuedAt, "",
	)
	if err != nil {
		t.Fatal(err)
	}

	logoutResult := make(chan error, 1)
	go func() { logoutResult <- svc.Logout(ctx, refreshToken, "") }()
	select {
	case <-cacheStore.started:
	case <-ctx.Done():
		t.Fatalf("logout did not reach revocation publication: %v", ctx.Err())
	}

	type refreshResult struct {
		accessToken  string
		refreshToken string
		err          error
	}
	refreshStarted := make(chan struct{})
	refreshed := make(chan refreshResult, 1)
	go func() {
		close(refreshStarted)
		accessToken, nextRefreshToken, err := svc.RefreshSession(
			ctx, refreshToken, token.AudienceApp, time.Now().UTC(),
		)
		refreshed <- refreshResult{accessToken: accessToken, refreshToken: nextRefreshToken, err: err}
	}()
	<-refreshStarted
	select {
	case result := <-refreshed:
		t.Fatalf("refresh escaped while logout held the user lock: %v", result.err)
	case <-time.After(150 * time.Millisecond):
	}

	close(cacheStore.release)
	released = true
	select {
	case err := <-logoutResult:
		if err != nil {
			t.Fatalf("Logout failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("logout timed out: %v", ctx.Err())
	}
	select {
	case result := <-refreshed:
		if !errors.Is(result.err, ErrInvalidRefreshToken) {
			t.Fatalf("refresh after logout = %v, want ErrInvalidRefreshToken", result.err)
		}
		if result.accessToken != "" || result.refreshToken != "" {
			t.Fatal("refresh returned tokens after concurrent logout")
		}
	case <-ctx.Done():
		t.Fatalf("refresh timed out: %v", ctx.Err())
	}
}

func TestLogoutWithOnlyAccessTokenRevokesThatToken(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "access-only-logout-" + uuid.NewString() + "@example.com",
			Username: "access-only-logout-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatal(err)
		}

		cacheStore := &sessionTestCache{values: map[string][]byte{}}
		svc := newDBSessionServiceWithCache(t, tdb, 10*time.Minute, cacheStore, 0)
		accessToken, _, err := svc.CreateSession(
			ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "logout-test", now, "",
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := svc.Logout(ctx, "", accessToken); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now); !errors.Is(err, token.ErrRevokedToken) {
			t.Fatalf("access-only logout validation = %v, want ErrRevokedToken", err)
		}
	})
}

func TestSwitchSessionOrganizationRotatesTokens(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "session-switch@example.com",
			Username: "session-switch",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		team, err := tdb.Store.CreateOrganization(ctx, domain.Organization{
			Name: "Team",
			Kind: domain.OrganizationKindTeam,
		})
		if err != nil {
			t.Fatalf("CreateOrganization failed: %v", err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: team.ID,
			UserID:         user.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatalf("CreateOrganizationMembership failed: %v", err)
		}

		svc := newDBSessionService(t, tdb, 10*time.Minute)
		_, oldRefreshToken, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "test-agent", now, "")
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		oldRT, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(oldRefreshToken))
		if err != nil {
			t.Fatalf("GetRefreshTokenByHash old failed: %v", err)
		}

		accessToken, refreshToken, err := svc.SwitchSessionOrganization(ctx, user.ID, oldRT.SessionID, team.ID, token.AudienceApp, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("SwitchSessionOrganization failed: %v", err)
		}

		identity, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now.Add(time.Minute))
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}
		if identity.OrganizationID != team.ID || identity.OrganizationRole != domain.OrganizationRoleMember {
			t.Fatalf("expected switched org/member role, got org=%q role=%q", identity.OrganizationID, identity.OrganizationRole)
		}

		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(oldRefreshToken)); !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("expected old refresh token deleted, got %v", err)
		}
		newRT, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
		if err != nil {
			t.Fatalf("GetRefreshTokenByHash new failed: %v", err)
		}
		if newRT.OrganizationID != team.ID {
			t.Fatalf("expected new refresh org %q, got %q", team.ID, newRT.OrganizationID)
		}
	})
}

func TestRefreshSessionReuseCommitsSessionRevocation(t *testing.T) {
	tests := []struct {
		name               string
		newCache           func() cache.Cache
		wantAccessTokenErr error
		wantReuseCacheErr  bool
	}{
		{
			name:               "noop cache",
			newCache:           func() cache.Cache { return cache.NewNoop() },
			wantAccessTokenErr: nil,
		},
		{
			name: "redis-like cache",
			newCache: func() cache.Cache {
				return &sessionTestCache{values: map[string][]byte{}}
			},
			wantAccessTokenErr: token.ErrRevokedToken,
		},
		{
			name: "unavailable cache",
			newCache: func() cache.Cache {
				return &sessionTestCache{err: errSessionTestCache}
			},
			wantAccessTokenErr: errSessionTestCache,
			wantReuseCacheErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdb := testutil.OpenTestDB(t)
			ctx := context.Background()
			now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
			suffix := uuid.NewString()

			user, err := tdb.Store.CreateUser(ctx, domain.User{
				Email:    "refresh-reuse-" + suffix + "@example.com",
				Username: "refresh-reuse-" + suffix,
			})
			if err != nil {
				t.Fatal(err)
			}
			org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = tdb.Store.DeleteUser(context.Background(), user.ID)
				_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
			})

			svc := newDBSessionServiceWithCache(t, tdb, 10*time.Minute, tt.newCache(), -time.Nanosecond)
			_, originalRefreshToken, err := svc.CreateSession(
				ctx,
				user.ID,
				token.AudienceApp,
				domain.AuthenticationMethodPassword,
				"reuse-test",
				now,
				"",
			)
			if err != nil {
				t.Fatal(err)
			}

			rotatedAt := now.Add(time.Minute)
			accessToken, descendantRefreshToken, err := svc.RefreshSession(
				ctx,
				originalRefreshToken,
				token.AudienceApp,
				rotatedAt,
			)
			if err != nil {
				t.Fatal(err)
			}
			if descendantRefreshToken == originalRefreshToken {
				t.Fatal("expected refresh token rotation")
			}

			original, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(originalRefreshToken))
			if err != nil {
				t.Fatal(err)
			}
			if original.ConsumedAt == nil {
				t.Fatal("expected original refresh token to be consumed")
			}

			reusedAt := now.Add(2 * time.Minute)
			gotAccessToken, gotRefreshToken, err := svc.RefreshSession(
				ctx,
				originalRefreshToken,
				token.AudienceApp,
				reusedAt,
			)
			if !errors.Is(err, ErrRefreshTokenReuse) {
				t.Fatalf("expected ErrRefreshTokenReuse, got %v", err)
			}
			if tt.wantReuseCacheErr && !errors.Is(err, errSessionTestCache) {
				t.Fatalf("expected cache error to be preserved, got %v", err)
			}
			if gotAccessToken != "" || gotRefreshToken != "" {
				t.Fatal("expected reuse response to omit tokens")
			}

			persistedSession, err := tdb.Store.GetSessionByID(context.Background(), original.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			if persistedSession.RevokedAt == nil || !persistedSession.RevokedAt.Equal(reusedAt) {
				t.Fatalf("session revoked_at = %v, want %v", persistedSession.RevokedAt, reusedAt)
			}
			reuseEvents, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
				Type:      domain.SecurityEventSessionRefreshTokenReuse,
				Outcome:   domain.SecurityEventOutcomeDenied,
				SessionID: &original.SessionID,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(reuseEvents) != 1 || reuseEvents[0].ReasonCode != domain.SecurityEventReasonRefreshTokenReuse || reuseEvents[0].UserID == nil || *reuseEvents[0].UserID != user.ID {
				t.Fatalf("unexpected refresh reuse event: %+v", reuseEvents)
			}

			_, accessErr := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, reusedAt)
			if !errors.Is(accessErr, tt.wantAccessTokenErr) {
				t.Fatalf("access token error = %v, want %v", accessErr, tt.wantAccessTokenErr)
			}

			_, _, err = svc.RefreshSession(
				ctx,
				descendantRefreshToken,
				token.AudienceApp,
				now.Add(3*time.Minute),
			)
			if !errors.Is(err, ErrInvalidRefreshToken) {
				t.Fatalf("expected descendant refresh token to be rejected, got %v", err)
			}
		})
	}
}

func TestRefreshSessionConcurrentRotationHasSingleWinner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	now := time.Date(2026, 9, 16, 14, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "concurrent-refresh-" + suffix + "@example.com",
		Username: "concurrent-refresh-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
	})

	svc := newDBSessionServiceWithCache(t, tdb, 10*time.Minute, cache.NewNoop(), -time.Nanosecond)
	type refreshResult struct {
		accessToken  string
		refreshToken string
		err          error
	}

	const attempts = 10
	for attempt := range attempts {
		attemptNow := now.Add(time.Duration(attempt) * time.Minute)
		_, originalRefreshToken, err := svc.CreateSession(
			ctx,
			user.ID,
			token.AudienceApp,
			domain.AuthenticationMethodPassword,
			"concurrent-refresh-test",
			attemptNow,
			"",
		)
		if err != nil {
			t.Fatalf("attempt %d: create session: %v", attempt, err)
		}
		original, err := tdb.Store.GetRefreshTokenByHash(ctx, hashRefreshToken(originalRefreshToken))
		if err != nil {
			t.Fatalf("attempt %d: get original refresh token: %v", attempt, err)
		}

		lockCtx, releaseLock, err := tdb.Tx.Begin(ctx)
		if err != nil {
			t.Fatalf("attempt %d: begin lock transaction: %v", attempt, err)
		}
		if _, err := tdb.Store.GetRefreshTokenByHashForUpdate(lockCtx, hashRefreshToken(originalRefreshToken)); err != nil {
			releaseLock()
			t.Fatalf("attempt %d: lock refresh token: %v", attempt, err)
		}

		start := make(chan struct{})
		results := make(chan refreshResult, 2)
		var ready sync.WaitGroup
		ready.Add(2)
		for range 2 {
			go func() {
				ready.Done()
				<-start
				accessToken, refreshToken, err := svc.RefreshSession(
					ctx,
					originalRefreshToken,
					token.AudienceApp,
					attemptNow.Add(time.Second),
				)
				results <- refreshResult{accessToken: accessToken, refreshToken: refreshToken, err: err}
			}()
		}
		ready.Wait()
		close(start)

		select {
		case result := <-results:
			_ = tdb.Tx.Rollback(lockCtx)
			releaseLock()
			t.Fatalf("attempt %d: refresh completed while token row was locked: %v", attempt, result.err)
		case <-time.After(50 * time.Millisecond):
		}
		if err := tdb.Tx.Commit(lockCtx); err != nil {
			releaseLock()
			t.Fatalf("attempt %d: release refresh-token lock: %v", attempt, err)
		}
		releaseLock()

		successes := 0
		reuses := 0
		for range 2 {
			select {
			case result := <-results:
				switch {
				case result.err == nil:
					successes++
					if result.accessToken == "" || result.refreshToken == "" || result.refreshToken == originalRefreshToken {
						t.Fatalf("attempt %d: successful rotation returned invalid tokens", attempt)
					}
				case errors.Is(result.err, ErrRefreshTokenReuse):
					reuses++
					if result.accessToken != "" || result.refreshToken != "" {
						t.Fatalf("attempt %d: losing rotation returned tokens", attempt)
					}
				default:
					t.Fatalf("attempt %d: concurrent rotation returned %v", attempt, result.err)
				}
			case <-ctx.Done():
				t.Fatalf("attempt %d: concurrent rotation timed out: %v", attempt, ctx.Err())
			}
		}
		if successes != 1 || reuses != 1 {
			t.Fatalf("attempt %d: successes=%d reuses=%d, want 1 each", attempt, successes, reuses)
		}

		var tokenCount int
		var unconsumedCount int
		if err := tdb.Store.DB().QueryRowContext(ctx, `
			SELECT count(*), count(*) FILTER (WHERE consumed_at IS NULL)
			FROM refresh_tokens
			WHERE session_id = $1
		`, original.SessionID).Scan(&tokenCount, &unconsumedCount); err != nil {
			t.Fatalf("attempt %d: count refresh-token family: %v", attempt, err)
		}
		if tokenCount != 2 || unconsumedCount != 1 {
			t.Fatalf(
				"attempt %d: refresh-token family has total=%d unconsumed=%d, want total=2 unconsumed=1",
				attempt,
				tokenCount,
				unconsumedCount,
			)
		}

		persistedSession, err := tdb.Store.GetSessionByID(ctx, original.SessionID)
		if err != nil {
			t.Fatalf("attempt %d: get session: %v", attempt, err)
		}
		if persistedSession.RevokedAt == nil {
			t.Fatalf("attempt %d: refresh-token reuse did not revoke the session", attempt)
		}
	}
}

func TestRefreshSessionWaitsForOrganizationLifecycleLock(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "session-lock-" + suffix + "@example.com",
		Username: "session-lock-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, user.ID, "Session Lock", domain.OrganizationKindTeam)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
	})

	svc := newDBSessionService(t, tdb, 10*time.Minute)
	_, refreshToken, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "test-agent", now, "")
	if err != nil {
		t.Fatal(err)
	}

	lockCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if _, err := tdb.Store.GetOrganizationByIDForUpdate(lockCtx, org.ID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, _, err := svc.RefreshSession(ctx, refreshToken, token.AudienceApp, now.Add(time.Minute))
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("refresh completed before organization lifecycle lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tdb.Tx.Commit(lockCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("refresh did not finish after organization lifecycle lock was released")
	}
}

func TestActiveSessionPreventsOrganizationMembershipRemoval(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "session-org-removed@example.com",
			Username: "session-org-removed",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		svc := newDBSessionService(t, tdb, 10*time.Minute)
		accessToken, _, err := svc.CreateSession(ctx, user.ID, token.AudienceApp, domain.AuthenticationMethodPassword, "test-agent", now, "")
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		identity, err := svc.ValidateAccessToken(ctx, accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}
		err = tdb.Store.DeleteOrganizationMembership(ctx, identity.OrganizationID, user.ID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "fk_sessions_active_organization_membership" {
			t.Fatalf("expected active session membership FK violation, got %v", err)
		}
	})
}

func TestValidateAccessToken_Succeeds(t *testing.T) {
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	svc := newTestSessionService(t, 10*time.Minute)

	userID := uuid.New()
	sessionID := uuid.New()
	organizationID := uuid.New()

	var rs roles.Roles
	rs.AddAdmin()
	rs.AddMonitor()

	accessToken, err := svc.accessTokens.Generate(
		userID,
		sessionID,
		organizationID,
		string(domain.OrganizationRoleOwner),
		token.AudienceAdmin,
		rs,
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	identity, err := svc.ValidateAccessToken(
		context.Background(),
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
	if identity.OrganizationID != organizationID {
		t.Fatalf("expected organization id %q, got %q", organizationID, identity.OrganizationID)
	}
	if identity.OrganizationRole != domain.OrganizationRoleOwner {
		t.Fatalf("expected organization role %q, got %q", domain.OrganizationRoleOwner, identity.OrganizationRole)
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
		uuid.New(),
		string(domain.OrganizationRoleOwner),
		token.AudienceApp,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAccessToken(
		context.Background(),
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
		context.Background(),
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
		uuid.New(),
		string(domain.OrganizationRoleOwner),
		token.AudienceApp,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAnyAccessToken(context.Background(), accessToken, now)
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
		uuid.New(),
		string(domain.OrganizationRoleOwner),
		token.AudienceAdmin,
		roles.Roles{},
		now,
	)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = svc.ValidateAnyAccessToken(context.Background(), accessToken, now)
	if err != nil {
		t.Fatalf("ValidateAnyAccessToken failed: %v", err)
	}
}

func TestIdentityFromClaims_InvalidSubject(t *testing.T) {
	svc := newTestSessionService(t, 10*time.Minute)

	claims := &token.AccessClaims{
		SessionID: uuid.New(),
		OrgID:     uuid.New(),
		OrgRole:   string(domain.OrganizationRoleOwner),
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
		OrgID:     uuid.New(),
		OrgRole:   string(domain.OrganizationRoleOwner),
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
		OrgID:     uuid.New(),
		OrgRole:   string(domain.OrganizationRoleOwner),
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
	organizationID := uuid.New()

	claims := &token.AccessClaims{
		SessionID: sessionID,
		OrgID:     organizationID,
		OrgRole:   string(domain.OrganizationRoleOwner),
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
	if identity.OrganizationID != organizationID {
		t.Fatalf("expected organization id %q, got %q", organizationID, identity.OrganizationID)
	}
	if identity.OrganizationRole != domain.OrganizationRoleOwner {
		t.Fatalf("expected organization role %q, got %q", domain.OrganizationRoleOwner, identity.OrganizationRole)
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
		{
			name: "admin audience denies operator",
			setup: func(r *roles.Roles) {
				r.AddOperator()
			},
			audience: token.AudienceAdmin,
			want:     false,
		},
		{
			name:     "operator audience denies empty roles",
			setup:    func(r *roles.Roles) {},
			audience: token.AudienceOperator,
			want:     false,
		},
		{
			name: "operator audience denies admin",
			setup: func(r *roles.Roles) {
				r.AddAdmin()
			},
			audience: token.AudienceOperator,
			want:     false,
		},
		{
			name: "operator audience allows operator",
			setup: func(r *roles.Roles) {
				r.AddOperator()
			},
			audience: token.AudienceOperator,
			want:     true,
		},
		{
			name: "operator audience allows additive operator role",
			setup: func(r *roles.Roles) {
				r.AddAdmin()
				r.AddOperator()
			},
			audience: token.AudienceOperator,
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
