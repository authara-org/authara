package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/google/uuid"
)

type AccessTokenRevocations struct {
	cache cache.Cache
	ttl   time.Duration
}

func NewAccessTokenRevocations(cache cache.Cache, ttl time.Duration) *AccessTokenRevocations {
	return &AccessTokenRevocations{cache: cache, ttl: ttl}
}

func (r *AccessTokenRevocations) RevokeToken(ctx context.Context, accessToken string, ttl time.Duration) error {
	if r == nil || r.cache == nil || ttl <= 0 {
		return nil
	}
	sum := sha256.Sum256([]byte(accessToken))
	return r.cache.Set(ctx, cache.RevokedAccessTokenKey(hex.EncodeToString(sum[:])), []byte("1"), ttl)
}

func (r *AccessTokenRevocations) RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	return r.revokeScope(ctx, cache.RevokedAccessTokenSessionKey(sessionID.String()), revokedAt)
}

func (r *AccessTokenRevocations) RevokeUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	return r.revokeScope(ctx, cache.RevokedAccessTokenUserKey(userID.String()), revokedAt)
}

func (r *AccessTokenRevocations) RevokeMembership(
	ctx context.Context,
	userID uuid.UUID,
	organizationID uuid.UUID,
	revokedAt time.Time,
) error {
	return r.revokeScope(
		ctx,
		cache.RevokedAccessTokenMembershipKey(userID.String(), organizationID.String()),
		revokedAt,
	)
}

func (r *AccessTokenRevocations) Check(
	ctx context.Context,
	accessToken string,
	claims *AccessClaims,
) error {
	if r == nil || r.cache == nil {
		return nil
	}
	if claims == nil || claims.IssuedAt == nil {
		return ErrInvalidClaims
	}

	sum := sha256.Sum256([]byte(accessToken))
	values, err := r.cache.GetMany(ctx,
		cache.RevokedAccessTokenKey(hex.EncodeToString(sum[:])),
		cache.RevokedAccessTokenSessionKey(claims.SessionID.String()),
		cache.RevokedAccessTokenUserKey(claims.Subject),
		cache.RevokedAccessTokenMembershipKey(claims.Subject, claims.OrgID.String()),
	)
	if err != nil {
		return fmt.Errorf("check access token revocation: %w", err)
	}
	if len(values) != 4 {
		return fmt.Errorf("check access token revocation: expected 4 values, got %d", len(values))
	}
	if values[0] != nil {
		return ErrRevokedToken
	}

	issuedAt := claims.IssuedAt.Time.UnixNano()
	for _, value := range values[1:] {
		if value == nil {
			continue
		}
		revokedAt, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return fmt.Errorf("check access token revocation: invalid cutoff: %w", err)
		}
		if issuedAt <= revokedAt {
			return ErrRevokedToken
		}
	}
	return nil
}

func (r *AccessTokenRevocations) revokeScope(ctx context.Context, key string, revokedAt time.Time) error {
	if r == nil || r.cache == nil {
		return nil
	}
	value := []byte(strconv.FormatInt(revokedAt.UnixNano(), 10))
	return r.cache.Set(ctx, key, value, r.ttl)
}
