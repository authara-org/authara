package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainOrganizationInvitation(m model.OrganizationInvitation) domain.OrganizationInvitation {
	return domain.OrganizationInvitation{
		ID:               m.ID,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		OrganizationID:   m.OrganizationID,
		Email:            m.Email,
		Role:             domain.OrganizationRole(m.Role),
		TokenHash:        m.TokenHash,
		InvitedByUserID:  m.InvitedByUserID,
		ExpiresAt:        m.ExpiresAt,
		AcceptedAt:       m.AcceptedAt,
		AcceptedByUserID: m.AcceptedByUserID,
		RevokedAt:        m.RevokedAt,
		RevokedByUserID:  m.RevokedByUserID,
	}
}

func toModelOrganizationInvitation(d domain.OrganizationInvitation) model.OrganizationInvitation {
	return model.OrganizationInvitation{
		OrganizationID:   d.OrganizationID,
		Email:            normalizeEmail(d.Email),
		Role:             string(d.Role),
		TokenHash:        d.TokenHash,
		InvitedByUserID:  d.InvitedByUserID,
		ExpiresAt:        d.ExpiresAt,
		AcceptedAt:       d.AcceptedAt,
		AcceptedByUserID: d.AcceptedByUserID,
		RevokedAt:        d.RevokedAt,
		RevokedByUserID:  d.RevokedByUserID,
	}
}

const organizationInvitationColumns = `
	id,
	created_at,
	updated_at,
	organization_id,
	email,
	role,
	token_hash,
	invited_by_user_id,
	expires_at,
	accepted_at,
	accepted_by_user_id,
	revoked_at,
	revoked_by_user_id
`

func scanOrganizationInvitation(row rowScanner, m *model.OrganizationInvitation) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.OrganizationID,
		&m.Email,
		&m.Role,
		&m.TokenHash,
		&m.InvitedByUserID,
		&m.ExpiresAt,
		&m.AcceptedAt,
		&m.AcceptedByUserID,
		&m.RevokedAt,
		&m.RevokedByUserID,
	)
}

func (s *Store) CreateOrganizationInvitation(ctx context.Context, invitation domain.OrganizationInvitation) (domain.OrganizationInvitation, error) {
	m := toModelOrganizationInvitation(invitation)

	if err := scanOrganizationInvitation(s.queryRow(ctx, `
		INSERT INTO organization_invitations (
			organization_id,
			email,
			role,
			token_hash,
			invited_by_user_id,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+organizationInvitationColumns,
		m.OrganizationID,
		m.Email,
		m.Role,
		m.TokenHash,
		m.InvitedByUserID,
		m.ExpiresAt,
	), &m); err != nil {
		return domain.OrganizationInvitation{}, err
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) GetOrganizationInvitationByTokenHashForUpdate(ctx context.Context, tokenHash string) (domain.OrganizationInvitation, error) {
	var m model.OrganizationInvitation

	err := scanOrganizationInvitation(s.queryRow(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash), &m)
	if err != nil {
		return domain.OrganizationInvitation{}, mapNoRows(err, ErrOrganizationInvitationNotFound)
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) GetOrganizationInvitationByTokenHash(ctx context.Context, tokenHash string) (domain.OrganizationInvitation, error) {
	var m model.OrganizationInvitation

	err := scanOrganizationInvitation(s.queryRow(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE token_hash = $1
	`, tokenHash), &m)
	if err != nil {
		return domain.OrganizationInvitation{}, mapNoRows(err, ErrOrganizationInvitationNotFound)
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) GetOrganizationInvitationByID(ctx context.Context, invitationID uuid.UUID) (domain.OrganizationInvitation, error) {
	var m model.OrganizationInvitation

	err := scanOrganizationInvitation(s.queryRow(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE id = $1
	`, invitationID), &m)
	if err != nil {
		return domain.OrganizationInvitation{}, mapNoRows(err, ErrOrganizationInvitationNotFound)
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) GetOrganizationInvitationByIDForUpdate(ctx context.Context, invitationID uuid.UUID) (domain.OrganizationInvitation, error) {
	var m model.OrganizationInvitation

	err := scanOrganizationInvitation(s.queryRow(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE id = $1
		FOR UPDATE
	`, invitationID), &m)
	if err != nil {
		return domain.OrganizationInvitation{}, mapNoRows(err, ErrOrganizationInvitationNotFound)
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) GetActiveOrganizationInvitationByOrganizationAndEmail(ctx context.Context, organizationID uuid.UUID, email string) (domain.OrganizationInvitation, error) {
	var m model.OrganizationInvitation

	err := scanOrganizationInvitation(s.queryRow(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE organization_id = $1
		  AND lower(email) = lower($2)
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, organizationID, normalizeEmail(email)), &m)
	if err != nil {
		return domain.OrganizationInvitation{}, mapNoRows(err, ErrOrganizationInvitationNotFound)
	}

	return toDomainOrganizationInvitation(m), nil
}

func (s *Store) MarkOrganizationInvitationAccepted(ctx context.Context, invitationID uuid.UUID, acceptedByUserID uuid.UUID, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE organization_invitations
		SET accepted_at = $2,
		    accepted_by_user_id = $3
		WHERE id = $1
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, invitationID, now, acceptedByUserID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOrganizationInvitationNotFound
	}
	return nil
}

func (s *Store) MarkOrganizationInvitationRevoked(ctx context.Context, invitationID uuid.UUID, revokedByUserID *uuid.UUID, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE organization_invitations
		SET revoked_at = $2,
		    revoked_by_user_id = $3
		WHERE id = $1
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`, invitationID, now, revokedByUserID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOrganizationInvitationNotFound
	}
	return nil
}

func (s *Store) ListOrganizationInvitationsByOrganizationID(ctx context.Context, organizationID uuid.UUID) ([]domain.OrganizationInvitation, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+organizationInvitationColumns+`
		FROM organization_invitations
		WHERE organization_id = $1
		ORDER BY created_at DESC, id DESC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.OrganizationInvitation, 0)
	for rows.Next() {
		var m model.OrganizationInvitation
		if err := scanOrganizationInvitation(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, toDomainOrganizationInvitation(m))
	}
	return out, rows.Err()
}
