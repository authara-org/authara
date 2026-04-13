package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/google/uuid"
)

type SessionConfig struct {
	Store                *store.Store
	Tx                   *tx.Manager
	AccessTokens         *token.AccessTokenService
	SessionTTL           time.Duration
	RefreshTokenTTL      time.Duration
	RefreshTokenRotation time.Duration
	AccessPolicy         accesspolicy.EmailAccessPolicy
}

type Service struct {
	store                *store.Store
	tx                   *tx.Manager
	accessTokens         *token.AccessTokenService
	sessionTTL           time.Duration
	refreshTokenTTL      time.Duration
	refreshTokenRotation time.Duration
	accessPolicy         accesspolicy.EmailAccessPolicy
}

func New(cfg SessionConfig) *Service {
	access := cfg.AccessPolicy
	if access == nil {
		access = accesspolicy.NoopEmailAccessPolicy{}
	}

	return &Service{
		store:                cfg.Store,
		tx:                   cfg.Tx,
		accessTokens:         cfg.AccessTokens,
		sessionTTL:           cfg.SessionTTL,
		refreshTokenTTL:      cfg.RefreshTokenTTL,
		refreshTokenRotation: cfg.RefreshTokenRotation,
		accessPolicy:         access,
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
		err = s.ensureUserAllowed(ctx, userID)
		if err != nil {
			return err
		}

		roleNames, err := s.store.GetUserPlatformRoleNames(ctx, userID)
		if err != nil {
			return err
		}

		platformRoles, err := roles.FromDBRoleNames(roleNames)
		if err != nil {
			return err
		}

		if !canAccessAudience(platformRoles, audience) {
			return ErrForbidden
		}

		disabled, err := s.store.IsUserDisabled(ctx, userID)
		if err != nil {
			return err
		}
		if disabled {
			return ErrUserDisabled
		}

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

		accessToken, err = s.accessTokens.Generate(
			userID,
			createdSession.ID,
			audience,
			platformRoles,
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

		err = s.ensureUserAllowed(ctx, session.UserID)
		if err != nil {
			return err
		}

		roleNames, err := s.store.GetUserPlatformRoleNames(ctx, session.UserID)
		if err != nil {
			return err
		}

		platformRoles, err := roles.FromDBRoleNames(roleNames)
		if err != nil {
			return err
		}

		if !canAccessAudience(platformRoles, audience) {
			return ErrForbidden
		}

		disabled, err := s.store.IsUserDisabled(ctx, session.UserID)
		if err != nil {
			return err
		}
		if disabled {
			return ErrUserDisabled
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

		newAccessToken, err = s.accessTokens.Generate(
			session.UserID,
			rt.SessionID,
			audience,
			platformRoles,
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

func (s *Service) CleanupExpiredData(ctx context.Context, now time.Time) error {
	err := s.store.DeleteExpiredSessions(ctx, now)
	if err != nil {
		return err
	}

	err = s.store.DeleteExpiredRefreshTokens(ctx, now)
	if err != nil {
		return err
	}
	return nil
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

func (s *Service) ValidateAccessToken(
	accessToken string,
	expectedAudience token.Audience,
	now time.Time,
) (*AccessIdentity, error) {
	claims, err := s.accessTokens.Parse(accessToken, expectedAudience, now)
	if err != nil {
		return nil, err
	}

	return s.identityFromClaims(claims)
}

func (s *Service) ValidateAnyAccessToken(
	accessToken string,
	now time.Time,
) (*AccessIdentity, error) {
	claims, err := s.accessTokens.ParseAny(accessToken, now)
	if err != nil {
		return nil, err
	}

	return s.identityFromClaims(claims)
}

func (s *Service) identityFromClaims(claims *token.AccessClaims) (*AccessIdentity, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil {
		return nil, token.ErrInvalidToken
	}

	rs, err := roles.FromClaims(claims.Roles)
	if err != nil {
		return nil, err
	}

	return &AccessIdentity{
		UserID:    userID,
		SessionID: claims.SessionID,
		Roles:     rs,
	}, nil
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

func (s *Service) ensureUserAllowed(ctx context.Context, userID uuid.UUID) error {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	allowed, err := s.accessPolicy.IsEmailAllowed(ctx, user.Email)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrUserNotAllowed
	}
	return nil
}

var audienceAccess = map[token.Audience][]roles.Role{
	token.AudienceAdmin: {
		roles.AutharaAdmin,
		roles.AutharaAuditor,
		roles.AutharaMonitor,
	},
}

func canAccessAudience(rs roles.Roles, audience token.Audience) bool {
	allowed, ok := audienceAccess[audience]
	if !ok {
		return true
	}
	return rs.HasAny(allowed...)
}

func (s *Service) ListUserSessions(
	ctx context.Context,
	userID uuid.UUID,
	currentSessionID uuid.UUID,
	now time.Time,
) ([]domain.Session, error) {
	sessions, err := s.store.ListActiveSessionsByUserID(ctx, userID, now)
	if err != nil {
		return nil, err
	}

	if currentSessionID == uuid.Nil {
		return sessions, nil
	}

	// Put current session first
	for i := range sessions {
		if sessions[i].ID == currentSessionID {
			if i == 0 {
				return sessions, nil
			}
			current := sessions[i]
			out := make([]domain.Session, 0, len(sessions))
			out = append(out, current)
			out = append(out, sessions[:i]...)
			out = append(out, sessions[i+1:]...)
			return out, nil
		}
	}

	return sessions, nil
}

func (s *Service) RevokeUserSession(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
) error {
	session, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}

	// Ownership check is the important security boundary
	if session.UserID != userID {
		return ErrForbidden
	}

	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.RevokeSession(txCtx, sessionID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensBySession(txCtx, sessionID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Service) RevokeOtherUserSessions(
	ctx context.Context,
	userID uuid.UUID,
	currentSessionID uuid.UUID,
	now time.Time,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.RevokeOtherSessionsByUserID(txCtx, userID, currentSessionID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensForOtherSessions(txCtx, userID, currentSessionID); err != nil {
			return err
		}
		return nil
	})
}
