package store

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func toDomainSession(m model.Session) domain.Session {
	return domain.Session{
		ID:     *m.ID,
		UserID: m.UserID,

		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		ExpiresAt: m.ExpiresAt,
		RevokedAt: m.RevokedAt,

		UserAgent: m.UserAgent,
	}
}

func toModelSession(d domain.Session) model.Session {
	return model.Session{
		ID:     nil,
		UserID: d.UserID,

		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,

		ExpiresAt: d.ExpiresAt,
		RevokedAt: d.RevokedAt,

		UserAgent: d.UserAgent,
	}
}

func toDomainRefreshToken(m model.RefreshToken) domain.RefreshToken {
	return domain.RefreshToken{
		ID:        *m.ID,
		SessionID: m.SessionID,

		TokenHash: m.TokenHash,

		CreatedAt:  m.CreatedAt,
		ExpiresAt:  m.ExpiresAt,
		ConsumedAt: m.ConsumedAt,
	}
}

func toModelRefreshToken(d domain.RefreshToken) model.RefreshToken {
	return model.RefreshToken{
		ID:        nil,
		SessionID: d.SessionID,

		TokenHash: d.TokenHash,

		CreatedAt:  d.CreatedAt,
		ExpiresAt:  d.ExpiresAt,
		ConsumedAt: d.ConsumedAt,
	}
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) (domain.Session, error) {
	m := toModelSession(session)

	err := s.query(ctx).
		Create(&m).
		Error

	if err != nil {
		return domain.Session{}, err
	}
	return toDomainSession(m), nil
}

func (s *Store) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	var m model.Session

	err := s.query(ctx).
		Where("id = ?", sessionID).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, ErrSessionNotFound
		}
		return domain.Session{}, err
	}

	return toDomainSession(m), nil
}

func (s *Store) RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	res := s.query(ctx).
		Model(&model.Session{}).
		Where("id = ?", sessionID).
		Update("revoked_at", revokedAt)

	if res.Error != nil {
		return res.Error
	}

	return nil
}

func (s *Store) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	res := s.query(ctx).
		Model(&model.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", revokedAt)

	if res.Error != nil {
		return res.Error
	}

	return nil
}

func (s *Store) CreateRefreshToken(ctx context.Context, token domain.RefreshToken) error {
	m := toModelRefreshToken(token)
	err := s.query(ctx).
		Create(&m).
		Error

	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetRefreshTokenByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	var m model.RefreshToken

	err := s.query(ctx).
		Where("token_hash = ?", hash).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RefreshToken{}, ErrRefreshTokenNotFound
		}
		return domain.RefreshToken{}, err
	}
	return toDomainRefreshToken(m), nil
}

func (s *Store) ConsumeRefreshToken(ctx context.Context, tokenID uuid.UUID, consumedAt time.Time) error {
	res := s.query(ctx).
		Model(&model.RefreshToken{}).
		Where("id = ? AND consumed_at IS NULL", tokenID).
		Update("consumed_at", consumedAt)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return ErrRefreshTokenNotFound
	}

	return nil
}

func (s *Store) DeleteRefreshTokensBySession(ctx context.Context, sessionID uuid.UUID) error {
	return s.query(ctx).
		Where("session_id = ?", sessionID.String()).
		Delete(&model.RefreshToken{}).
		Error
}

func (s *Store) DeleteExpiredRefreshTokens(ctx context.Context, now time.Time) error {
	db := s.query(ctx)

	err := db.
		Where("expires_at < ?", now).
		Delete(&model.RefreshToken{}).
		Error
	if err != nil {
		return err
	}

	return db.
		Where("consumed_at IS NOT NULL").
		Delete(&model.RefreshToken{}).
		Error
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	db := s.query(ctx)

	err := db.
		Where("expires_at < ?", now).
		Delete(&model.Session{}).
		Error
	if err != nil {
		return err
	}

	return db.
		Where("revoked_at IS NOT NULL").
		Delete(&model.Session{}).
		Error
}

func (s *Store) ListActiveSessionsByUserID(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Session, error) {
	var rows []model.Session

	err := s.query(ctx).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, now).
		Order("created_at DESC").
		Find(&rows).
		Error
	if err != nil {
		return nil, err
	}

	out := make([]domain.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainSession(row))
	}

	return out, nil
}

func (s *Store) GetActiveSessionByID(ctx context.Context, sessionID uuid.UUID, now time.Time) (domain.Session, error) {
	var m model.Session

	err := s.query(ctx).
		Where("id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, now).
		First(&m).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, ErrSessionNotFound
		}
		return domain.Session{}, err
	}

	return toDomainSession(m), nil
}

func (s *Store) RevokeOtherSessionsByUserID(ctx context.Context, userID uuid.UUID, keepSessionID uuid.UUID, revokedAt time.Time) error {
	res := s.query(ctx).
		Model(&model.Session{}).
		Where("user_id = ? AND id <> ? AND revoked_at IS NULL", userID, keepSessionID).
		Update("revoked_at", revokedAt)

	if res.Error != nil {
		return res.Error
	}

	return nil
}

func (s *Store) DeleteRefreshTokensForOtherSessions(ctx context.Context, userID uuid.UUID, keepSessionID uuid.UUID) error {
	sub := s.query(ctx).
		Model(&model.Session{}).
		Select("id").
		Where("user_id = ? AND id <> ?", userID, keepSessionID)

	return s.query(ctx).
		Where("session_id IN (?)", sub).
		Delete(&model.RefreshToken{}).
		Error
}

func (s *Store) DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	sub := s.query(ctx).
		Model(&model.Session{}).
		Select("id").
		Where("user_id = ?", userID)

	return s.query(ctx).
		Where("session_id IN (?)", sub).
		Delete(&model.RefreshToken{}).
		Error
}
