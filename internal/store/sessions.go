package store

import (
	"context"
	"errors"

	"github.com/alexlup06/authgate/internal/domain"
	"github.com/alexlup06/authgate/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func toDomainSession(m model.Session) domain.Session {
	return domain.Session{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		UserID: uuid.MustParse(m.UserID),

		RefreshToken: m.RefreshToken,
		IssuedAt:     m.IssuedAt,
		ExpiresAt:    m.ExpiresAt,
		Revoked:      m.Revoked,

		UserAgent: m.UserAgent,
	}
}

func toModelSession(d domain.Session) model.Session {
	return model.Session{
		ID:           d.ID,
		UserID:       d.UserID.String(),
		RefreshToken: d.RefreshToken,
		IssuedAt:     d.IssuedAt,
		ExpiresAt:    d.ExpiresAt,
		Revoked:      d.Revoked,
		UserAgent:    d.UserAgent,
	}
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) (domain.Session, error) {
	m := toModelSession(session)

	err := s.dbFromContext(ctx).
		Create(&m).
		Error

	if err != nil {
		return domain.Session{}, err
	}
	return toDomainSession(m), nil
}

func (s *Store) GetSessionByRefreshToken(ctx context.Context, token string) (domain.Session, error) {
	var m model.Session

	err := s.dbFromContext(ctx).
		Where("refresh_token = ?", token).
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

func (s *Store) RevokeSession(ctx context.Context, sessionID uuid.UUIDs) error {
	res := s.dbFromContext(ctx).
		Model(&model.Session{}).
		Where("id = ? AND revoked = false").
		Update("revoked", true)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (s *Store) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID) error {
	res := s.dbFromContext(ctx).
		Model(&model.Session{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true)

	if res.Error != nil {
		return res.Error
	}

	return nil
}
