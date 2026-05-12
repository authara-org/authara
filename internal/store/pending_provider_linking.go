package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainPendingProviderLink(m model.PendingProviderLink) domain.PendingProviderLink {
	return domain.PendingProviderLink{
		ID:        m.ID,
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

const pendingProviderLinkColumns = `
	id,
	user_id,
	session_id,
	challenge_id,
	provider,
	provider_user_id,
	provider_email,
	provider_email_verified,
	purpose,
	expires_at,
	consumed_at,
	created_at
`

func scanPendingProviderLink(row rowScanner, m *model.PendingProviderLink) error {
	return row.Scan(
		&m.ID,
		&m.UserID,
		&m.SessionID,
		&m.ChallengeID,
		&m.Provider,
		&m.ProviderUserID,
		&m.ProviderEmail,
		&m.ProviderEmailVerified,
		&m.Purpose,
		&m.ExpiresAt,
		&m.ConsumedAt,
		&m.CreatedAt,
	)
}

func (s *Store) CreatePendingProviderLink(ctx context.Context, link domain.PendingProviderLink) (domain.PendingProviderLink, error) {
	m := toModelPendingProviderLink(link)

	if err := scanPendingProviderLink(s.queryRow(ctx, `
		INSERT INTO pending_provider_links (
			user_id,
			session_id,
			challenge_id,
			provider,
			provider_user_id,
			provider_email,
			provider_email_verified,
			purpose,
			expires_at,
			consumed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+pendingProviderLinkColumns,
		m.UserID,
		m.SessionID,
		m.ChallengeID,
		m.Provider,
		m.ProviderUserID,
		m.ProviderEmail,
		m.ProviderEmailVerified,
		m.Purpose,
		m.ExpiresAt,
		m.ConsumedAt,
	), &m); err != nil {
		return domain.PendingProviderLink{}, err
	}

	return toDomainPendingProviderLink(m), nil
}

func (s *Store) GetPendingProviderLinkByID(ctx context.Context, id uuid.UUID) (domain.PendingProviderLink, error) {
	var m model.PendingProviderLink

	err := scanPendingProviderLink(s.queryRow(ctx, `
		SELECT `+pendingProviderLinkColumns+`
		FROM pending_provider_links
		WHERE id = $1
	`, id), &m)
	if err != nil {
		return domain.PendingProviderLink{}, mapNoRows(err, ErrorPendingProviderLinkNotFound)
	}

	return toDomainPendingProviderLink(m), nil
}

func (s *Store) ConsumePendingProviderLink(ctx context.Context, id uuid.UUID, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE pending_provider_links
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL AND expires_at > $1
	`, now, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
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
	res, err := s.exec(ctx, `
		UPDATE pending_provider_links
		SET provider_user_id = $1,
		    provider_email = $2,
		    provider_email_verified = $3
		WHERE id = $4 AND consumed_at IS NULL
	`, providerUserID, providerEmail, providerEmailVerified, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrorPendingProviderLinkNotFound
	}

	return nil
}
