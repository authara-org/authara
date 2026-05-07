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

func toDomainPendingProviderLink(m model.PendingProviderLink) domain.PendingProviderLink {
	return domain.PendingProviderLink{
		ID:        *m.ID,
		CreatedAt: m.CreatedAt,

		UserID:      m.UserID,
		SessionID:   m.SessionID,
		ChallengeID: m.ChallengeID,
		Provider:    domain.Provider(m.Provider),

		ProviderUserID:        m.ProviderUserID,
		ProviderEmail:         m.ProviderEmail,
		ProviderEmailVerified: m.ProviderEmailVerified,
		Purpose:               domain.PendingProviderLinkPurpose(m.Purpose),

		ExpiresAt:  m.ExpiresAt,
		ConsumedAt: m.ConsumedAt,
	}
}

func toModelPendingProviderLink(d domain.PendingProviderLink) model.PendingProviderLink {
	return model.PendingProviderLink{
		ID:                    nil,
		UserID:                d.UserID,
		SessionID:             d.SessionID,
		ChallengeID:           d.ChallengeID,
		Provider:              string(d.Provider),
		ProviderUserID:        d.ProviderUserID,
		ProviderEmail:         d.ProviderEmail,
		ProviderEmailVerified: d.ProviderEmailVerified,
		Purpose:               string(d.Purpose),
		ExpiresAt:             d.ExpiresAt,
		ConsumedAt:            d.ConsumedAt,
	}
}

func (s *Store) CreatePendingProviderLink(ctx context.Context, link domain.PendingProviderLink) (domain.PendingProviderLink, error) {
	m := toModelPendingProviderLink(link)

	err := s.query(ctx).
		Create(&m).
		Error
	if err != nil {
		return domain.PendingProviderLink{}, err
	}

	return toDomainPendingProviderLink(m), nil
}

func (s *Store) GetPendingProviderLinkByID(ctx context.Context, id uuid.UUID) (domain.PendingProviderLink, error) {
	var m model.PendingProviderLink

	err := s.query(ctx).
		Where("id = ?", id).
		First(&m).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PendingProviderLink{}, ErrorPendingProviderLinkNotFound
		}
		return domain.PendingProviderLink{}, err
	}

	return toDomainPendingProviderLink(m), nil
}

func (s *Store) ConsumePendingProviderLink(ctx context.Context, id uuid.UUID, now time.Time) error {
	res := s.query(ctx).
		Model(&model.PendingProviderLink{}).
		Where("id = ? AND consumed_at IS NULL AND expires_at > ?", id, now).
		Update("consumed_at", now)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrorPendingProviderLinkNotFound
	}

	return nil
}

func (s *Store) UpdatePendingProviderLinkOAuthIdentity(
	ctx context.Context,
	id uuid.UUID,
	providerUserID string,
	providerEmail string,
	providerEmailVerified bool,
) error {
	res := s.query(ctx).
		Model(&model.PendingProviderLink{}).
		Where("id = ? AND consumed_at IS NULL", id).
		Updates(map[string]any{
			"provider_user_id":        providerUserID,
			"provider_email":          providerEmail,
			"provider_email_verified": providerEmailVerified,
		})

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrorPendingProviderLinkNotFound
	}

	return nil
}
