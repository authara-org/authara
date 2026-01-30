package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/domain"
	"github.com/alexlup06-authgate/authgate/internal/session/token"
	"github.com/alexlup06-authgate/authgate/internal/store"
	"github.com/alexlup06-authgate/authgate/internal/store/tx"
	"github.com/google/uuid"
)

type SessionConfig struct {
	Store                *store.Store
	Tx                   *tx.Manager
	AccessTokens         *token.AccessTokenService
	SessionTTL           time.Duration
	RefreshTokenTTL      time.Duration
	RefreshTokenRotation time.Duration
}

type Service struct {
	store                *store.Store
	tx                   *tx.Manager
	accessTokens         *token.AccessTokenService
	sessionTTL           time.Duration
	refreshTokenTTL      time.Duration
	refreshTokenRotation time.Duration
}

func New(cfg SessionConfig) *Service {
	return &Service{
		store:                cfg.Store,
		tx:                   cfg.Tx,
		accessTokens:         cfg.AccessTokens,
		sessionTTL:           cfg.SessionTTL,
		refreshTokenTTL:      cfg.RefreshTokenTTL,
		refreshTokenRotation: cfg.RefreshTokenRotation,
	}
}

func (s *Service) CreateSession(
	ctx context.Context,
	userID uuid.UUID,
	audience token.Audience,
	userAgent string,
	now time.Time,
) (
	accessToken string,
	refreshToken string,
	err error,
) {
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		session := domain.Session{
			UserID:    userID,
			UserAgent: userAgent,
			ExpiresAt: now.Add(s.sessionTTL),
		}

		createdSession, err := s.store.CreateSession(ctx, session)
		if err != nil {
			return err
		}

		refreshToken, err = generateRefreshToken()
		if err != nil {
			return err
		}

		hashedRefreshToken := hashRefreshToken(refreshToken)

		rt := domain.RefreshToken{
			SessionID: createdSession.ID,
			TokenHash: hashedRefreshToken,
			ExpiresAt: now.Add(s.refreshTokenTTL),
		}

		err = s.store.CreateRefreshToken(ctx, rt)
		if err != nil {
			return err
		}

		roles := []string{}

		isAdmin, err := s.store.IsUserAdmin(ctx, userID)
		if err != nil {
			return err
		}

		if audience == token.AudienceAdmin && !isAdmin {
			return ErrForbidden
		}

		if isAdmin {
			roles = append(roles, "authgate:admin")
		}

		accessToken, err = s.accessTokens.Generate(
			userID,
			createdSession.ID,
			audience,
			roles,
			now,
		)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string, audience token.Audience, now time.Time) (newAccessToken string, newRefreshToken string, err error) {
	err = s.tx.WithTransaction(ctx, func(ctx context.Context) error {
		hashed := hashRefreshToken(refreshToken)

		rt, err := s.store.GetRefreshTokenByHash(ctx, hashed)
		if err != nil {
			return ErrInvalidRefreshToken
		}

		if rt.ConsumedAt != nil {
			_ = s.store.RevokeSession(ctx, rt.SessionID, now)
			return ErrRefreshTokenReuse
		}

		if rt.ExpiresAt.Before(now) {
			return ErrInvalidRefreshToken
		}

		session, err := s.store.GetSessionByID(ctx, rt.SessionID)
		if err != nil {
			return ErrInvalidRefreshToken
		}

		if session.ExpiresAt.Before(now) || session.RevokedAt != nil {
			return ErrInvalidRefreshToken
		}

		needToRotate := shouldRotate(rt, now, s.refreshTokenRotation)
		if needToRotate {
			err = s.store.ConsumeRefreshToken(ctx, rt.ID, now)
			if err != nil {
				return err
			}

			newRefreshToken, err = generateRefreshToken()
			if err != nil {
				return err
			}

			newHashed := hashRefreshToken(newRefreshToken)
			newExpiresAt := now.Add(s.refreshTokenTTL)
			if newExpiresAt.After(session.ExpiresAt) {
				newExpiresAt = session.ExpiresAt
			}

			newRT := domain.RefreshToken{
				SessionID: rt.SessionID,
				TokenHash: newHashed,
				CreatedAt: now,
				ExpiresAt: newExpiresAt,
			}

			err = s.store.CreateRefreshToken(ctx, newRT)
			if err != nil {
				return err
			}
		} else {
			newRefreshToken = refreshToken
		}

		roles := []string{}

		isAdmin, err := s.store.IsUserAdmin(ctx, session.UserID)
		if err != nil {
			return err
		}

		if audience == token.AudienceAdmin && !isAdmin {
			return ErrForbidden
		}

		if isAdmin {
			roles = append(roles, "authgate:admin")
		}

		newAccessToken, err = s.accessTokens.Generate(
			session.UserID,
			rt.SessionID,
			audience,
			roles,
			now,
		)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	hashed := hashRefreshToken(refreshToken)

	rt, err := s.store.GetRefreshTokenByHash(ctx, hashed)
	if err != nil {
		// Token missing, expired, already cleaned up
		// Logout must still succeed
		return nil
	}

	_ = s.store.RevokeSession(ctx, rt.SessionID, time.Now())

	return nil
}

func (s *Service) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	err := s.store.RevokeAllSessionsForUser(
		ctx,
		userID,
		time.Now(),
	)
	return err
}

// for SDK
func (s *Service) ValidateAccessToken(ctx context.Context, accessToken string, now time.Time) (userID uuid.UUID, err error) {
	return uuid.UUID{}, nil
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func shouldRotate(rt domain.RefreshToken, now time.Time, rotation time.Duration) bool {
	switch {
	case rotation < 0:
		return true
	case rotation == 0:
		return false
	default:
		return now.Sub(rt.CreatedAt) >= rotation
	}
}
