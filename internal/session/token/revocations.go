package token

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/google/uuid"
)

type AccessTokenRevocations struct {
	cache       cache.Cache
	ttlProvider func() time.Duration
}

func NewAccessTokenRevocations(cache cache.Cache, ttl time.Duration) *AccessTokenRevocations {
	return NewAccessTokenRevocationsWithTTL(cache, func() time.Duration { return ttl })
}

func NewAccessTokenRevocationsWithTTL(cache cache.Cache, ttl func() time.Duration) *AccessTokenRevocations {
	return &AccessTokenRevocations{cache: cache, ttlProvider: ttl}
}

func (r *AccessTokenRevocations) RevokeToken(ctx context.Context, claims *AccessClaims, ttl time.Duration) error {
	if r == nil || r.cache == nil || ttl <= 0 {
		return nil
	}

	tokenIdentifier, err := accessTokenIdentifier(claims)
	if err != nil {
		return err
	}
	if err := r.cache.Set(ctx, cache.RevokedAccessTokenKey(tokenIdentifier), []byte("1"), ttl); err != nil {
		return fmt.Errorf("%w: revoke access token: %w", ErrRevocationStoreUnavailable, err)
	}
	return nil
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
	_ string,
	claims *AccessClaims,
) error {
	if r == nil || r.cache == nil {
		return nil
	}
	if claims == nil || claims.IssuedAt == nil {
		return ErrInvalidClaims
	}
	tokenIdentifier, err := accessTokenIdentifier(claims)
	if err != nil {
		return err
	}

	values, err := r.cache.GetMany(ctx,
		cache.RevokedAccessTokenKey(tokenIdentifier),
		cache.RevokedAccessTokenSessionKey(claims.SessionID.String()),
		cache.RevokedAccessTokenUserKey(claims.Subject),
		cache.RevokedAccessTokenMembershipKey(claims.Subject, claims.OrgID.String()),
	)
	if err != nil {
		return fmt.Errorf("%w: check access token revocation: %w", ErrRevocationStoreUnavailable, err)
	}
	if len(values) != 4 {
		return fmt.Errorf(
			"%w: check access token revocation: expected 4 values, got %d",
			ErrRevocationStoreUnavailable,
			len(values),
		)
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
			return fmt.Errorf(
				"%w: check access token revocation: invalid cutoff: %w",
				ErrRevocationStoreUnavailable,
				err,
			)
		}
		if issuedAt <= revokedAt {
			return ErrRevokedToken
		}
	}
	return nil
}

func accessTokenIdentifier(claims *AccessClaims) (string, error) {
	if claims == nil {
		return "", ErrInvalidClaims
	}
	if claims.ID != "" {
		id, err := uuid.Parse(claims.ID)
		if err != nil {
			return "", ErrInvalidClaims
		}
		return id.String(), nil
	}
	if claims.SessionID == uuid.Nil || claims.OrgID == uuid.Nil || claims.Subject == "" || claims.IssuedAt == nil {
		return "", ErrInvalidClaims
	}

	// Tokens created before jti support are deterministic within these public
	// claims. Keep them revocable during a rolling v0.x upgrade without storing
	// any bearer-token material in the cache.
	return strings.Join([]string{
		"legacy",
		claims.SessionID.String(),
		claims.Subject,
		claims.OrgID.String(),
		strconv.FormatInt(claims.IssuedAt.Time.Unix(), 10),
		strings.Join(claims.Audience, ","),
	}, ":"), nil
}

func (r *AccessTokenRevocations) revokeScope(ctx context.Context, key string, revokedAt time.Time) error {
	if r == nil || r.cache == nil {
		return nil
	}
	if err := r.cache.SetMaxInt64(ctx, key, revokedAt.UnixNano(), r.ttlProvider()); err != nil {
		return fmt.Errorf("%w: write access-token revocation: %w", ErrRevocationStoreUnavailable, err)
	}
	return nil
}
