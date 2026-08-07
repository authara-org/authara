package token

import (
	"context"
	"errors"
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

type revocationTestCache struct {
	values map[string][]byte
	ttls   map[string]time.Duration
}

func (c *revocationTestCache) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := c.values[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return value, nil
}

func (c *revocationTestCache) GetMany(_ context.Context, keys ...string) ([][]byte, error) {
	values := make([][]byte, len(keys))
	for i, key := range keys {
		values[i] = c.values[key]
	}
	return values, nil
}

func (c *revocationTestCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	c.values[key] = value
	c.ttls[key] = ttl
	return nil
}

func (c *revocationTestCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	return nil
}

func (c *revocationTestCache) Close() error { return nil }
