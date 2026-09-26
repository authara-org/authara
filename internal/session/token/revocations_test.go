package token

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestAccessTokenRevocations(t *testing.T) {
	ctx := context.Background()
	store := &revocationTestCache{values: map[string][]byte{}, ttls: map[string]time.Duration{}}
	revocations := NewAccessTokenRevocations(store, 10*time.Minute)
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	claims := &AccessClaims{
		SessionID: uuid.New(),
		OrgID:     uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  uuid.NewString(),
			IssuedAt: jwt.NewNumericDate(now),
		},
	}

	if err := revocations.RevokeToken(ctx, "secret-token", 3*time.Minute); err != nil {
		t.Fatalf("revoke token failed: %v", err)
	}
	if err := revocations.Check(ctx, "secret-token", claims); !errors.Is(err, ErrRevokedToken) {
		t.Fatalf("expected exact token to be revoked, got %v", err)
	}

	if err := revocations.RevokeMembership(ctx, uuid.MustParse(claims.Subject), claims.OrgID, now); err != nil {
		t.Fatalf("revoke membership failed: %v", err)
	}
	if err := revocations.Check(ctx, "another-token", claims); !errors.Is(err, ErrRevokedToken) {
		t.Fatalf("expected membership token to be revoked, got %v", err)
	}
	freshClaims := *claims
	freshClaims.IssuedAt = jwt.NewNumericDate(now.Add(time.Second))
	if err := revocations.Check(ctx, "fresh-token", &freshClaims); err != nil {
		t.Fatalf("expected token issued after revocation to remain valid, got %v", err)
	}

	for key, ttl := range store.ttls {
		if strings.Contains(key, "secret-token") || string(store.values[key]) == "secret-token" {
			t.Fatalf("bearer token was stored in Redis entry %q", key)
		}
		if ttl != 3*time.Minute && ttl != 10*time.Minute {
			t.Fatalf("unexpected TTL %s for %q", ttl, key)
		}
	}
}

func TestAccessTokenRevocationsReadsCurrentTTLForEachScope(t *testing.T) {
	store := &revocationTestCache{values: map[string][]byte{}, ttls: map[string]time.Duration{}}
	ttl := 10 * time.Minute
	revocations := NewAccessTokenRevocationsWithTTL(store, func() time.Duration { return ttl })
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)

	first := uuid.New()
	if err := revocations.RevokeSession(context.Background(), first, now); err != nil {
		t.Fatal(err)
	}
	if got := store.ttls[cache.RevokedAccessTokenSessionKey(first.String())]; got != 10*time.Minute {
		t.Fatalf("first revocation TTL = %s", got)
	}

	ttl = 45 * time.Minute
	second := uuid.New()
	if err := revocations.RevokeSession(context.Background(), second, now); err != nil {
		t.Fatal(err)
	}
	if got := store.ttls[cache.RevokedAccessTokenSessionKey(second.String())]; got != 45*time.Minute {
		t.Fatalf("updated revocation TTL = %s", got)
	}
}

func TestAccessTokenRevocationsKeepNewestScopeCutoff(t *testing.T) {
	store := &revocationTestCache{values: map[string][]byte{}, ttls: map[string]time.Duration{}}
	revocations := NewAccessTokenRevocations(store, 10*time.Minute)
	userID := uuid.New()
	base := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

	if err := revocations.RevokeUser(context.Background(), userID, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := revocations.RevokeUser(context.Background(), userID, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	claims := &AccessClaims{
		SessionID: uuid.New(),
		OrgID:     uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  userID.String(),
			IssuedAt: jwt.NewNumericDate(base.Add(90 * time.Second)),
		},
	}
	if err := revocations.Check(context.Background(), "token", claims); !errors.Is(err, ErrRevokedToken) {
		t.Fatalf("Check error = %v, want ErrRevokedToken after delayed older write", err)
	}
}

func TestAccessTokenRevocationsClassifiesStoreFailures(t *testing.T) {
	storeFailure := errors.New("redis unavailable")
	store := &revocationTestCache{
		values: map[string][]byte{}, ttls: map[string]time.Duration{}, err: storeFailure,
	}
	revocations := NewAccessTokenRevocations(store, 10*time.Minute)
	claims := &AccessClaims{
		SessionID: uuid.New(),
		OrgID:     uuid.New(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  uuid.NewString(),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	for name, err := range map[string]error{
		"read":  revocations.Check(context.Background(), "token", claims),
		"write": revocations.RevokeUser(context.Background(), uuid.New(), time.Now()),
	} {
		if !errors.Is(err, ErrRevocationStoreUnavailable) || !errors.Is(err, storeFailure) {
			t.Errorf("%s error = %v, want revocation-store and root causes", name, err)
		}
	}
}

type revocationTestCache struct {
	values map[string][]byte
	ttls   map[string]time.Duration
	err    error
}

func (c *revocationTestCache) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := c.values[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return value, nil
}

func (c *revocationTestCache) GetMany(_ context.Context, keys ...string) ([][]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	values := make([][]byte, len(keys))
	for i, key := range keys {
		values[i] = c.values[key]
	}
	return values, nil
}

func (c *revocationTestCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if c.err != nil {
		return c.err
	}
	c.values[key] = value
	c.ttls[key] = ttl
	return nil
}

func (c *revocationTestCache) SetMaxInt64(_ context.Context, key string, value int64, ttl time.Duration) error {
	if c.err != nil {
		return c.err
	}
	if current, ok := c.values[key]; ok {
		currentValue, err := strconv.ParseInt(string(current), 10, 64)
		if err != nil {
			return err
		}
		if currentValue >= value {
			if c.ttls[key] < ttl {
				c.ttls[key] = ttl
			}
			return nil
		}
	}
	c.values[key] = []byte(strconv.FormatInt(value, 10))
	c.ttls[key] = ttl
	return nil
}

func (c *revocationTestCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	return nil
}

func (c *revocationTestCache) Close() error { return nil }
