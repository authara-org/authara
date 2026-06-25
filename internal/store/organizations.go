package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func (s *Store) EnsureOrganizationMode(ctx context.Context, mode string) error {
	var stored string
	if _, err := s.exec(ctx, `
		INSERT INTO organization_mode (mode)
		VALUES ($1)
		ON CONFLICT (id) DO NOTHING
	`, mode); err != nil {
		return err
	}
	if err := s.queryRow(ctx, `SELECT mode FROM organization_mode WHERE id = 1`).Scan(&stored); err != nil {
		return err
	}
	if stored != mode {
		return fmt.Errorf("organization mode mismatch: database=%q config=%q", stored, mode)
	}
	return nil
}

func toDomainOrganization(m model.Organization) domain.Organization {
	return domain.Organization{
		ID:              m.ID,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		Name:            m.Name,
		Kind:            domain.OrganizationKind(m.Kind),
		CreatedByUserID: m.CreatedByUserID,
	}
}

func toModelOrganization(d domain.Organization) model.Organization {
	return model.Organization{
		Name:            d.Name,
		Kind:            string(d.Kind),
		CreatedByUserID: d.CreatedByUserID,
	}
}

func toDomainOrganizationMembership(m model.OrganizationMembership) domain.OrganizationMembership {
	return domain.OrganizationMembership{
		OrganizationID: m.OrganizationID,
		UserID:         m.UserID,
		Role:           domain.OrganizationRole(m.Role),
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func toModelOrganizationMembership(d domain.OrganizationMembership) model.OrganizationMembership {
	return model.OrganizationMembership{
		OrganizationID: d.OrganizationID,
		UserID:         d.UserID,
		Role:           string(d.Role),
	}
}

const organizationColumns = `
	id,
	created_at,
	updated_at,
	name,
	kind,
	created_by_user_id
`

func scanOrganization(row rowScanner, m *model.Organization) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.Name,
		&m.Kind,
		&m.CreatedByUserID,
	)
}

const organizationMembershipColumns = `
	organization_id,
	user_id,
	role,
	created_at,
	updated_at
`

func scanOrganizationMembership(row rowScanner, m *model.OrganizationMembership) error {
	return row.Scan(
		&m.OrganizationID,
		&m.UserID,
		&m.Role,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
}

func (s *Store) CreateOrganization(ctx context.Context, org domain.Organization) (domain.Organization, error) {
	var err error
	org.Name, err = normalizeOrganizationName(org.Name)
	if err != nil {
		return domain.Organization{}, err
	}

	m := toModelOrganization(org)

	if err := scanOrganization(s.queryRow(ctx, `
		INSERT INTO organizations (name, kind, created_by_user_id)
		VALUES ($1, $2, $3)
		RETURNING `+organizationColumns,
		m.Name,
		m.Kind,
		m.CreatedByUserID,
	), &m); err != nil {
		return domain.Organization{}, err
	}

	return toDomainOrganization(m), nil
}

func (s *Store) GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	var m model.Organization

	err := scanOrganization(s.queryRow(ctx, `SELECT `+organizationColumns+` FROM organizations WHERE id = $1`, organizationID), &m)
	if err != nil {
		return domain.Organization{}, mapNoRows(err, ErrOrganizationNotFound)
	}

	return toDomainOrganization(m), nil
}

func (s *Store) UpdateOrganizationName(ctx context.Context, organizationID uuid.UUID, name string) (domain.Organization, error) {
	var m model.Organization
	name, err := normalizeOrganizationName(name)
	if err != nil {
		return domain.Organization{}, err
	}

	err = scanOrganization(s.queryRow(ctx, `
		UPDATE organizations
		SET name = $2
		WHERE id = $1
		RETURNING `+organizationColumns,
		organizationID,
		name,
	), &m)
	if err != nil {
		return domain.Organization{}, mapNoRows(err, ErrOrganizationNotFound)
	}

	return toDomainOrganization(m), nil
}

func (s *Store) CreateOrganizationMembership(ctx context.Context, membership domain.OrganizationMembership) (domain.OrganizationMembership, error) {
	m := toModelOrganizationMembership(membership)

	if err := scanOrganizationMembership(s.queryRow(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING `+organizationMembershipColumns,
		m.OrganizationID,
		m.UserID,
		m.Role,
	), &m); err != nil {
		return domain.OrganizationMembership{}, err
	}

	return toDomainOrganizationMembership(m), nil
}

func (s *Store) GetOrganizationMembership(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID) (domain.OrganizationMembership, error) {
	var m model.OrganizationMembership

	err := scanOrganizationMembership(s.queryRow(ctx, `
		SELECT `+organizationMembershipColumns+`
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2
	`, organizationID, userID), &m)
	if err != nil {
		return domain.OrganizationMembership{}, mapNoRows(err, ErrOrganizationMembershipNotFound)
	}

	return toDomainOrganizationMembership(m), nil
}

func (s *Store) ListOrganizationMembershipsByOrganizationID(ctx context.Context, organizationID uuid.UUID) ([]domain.OrganizationMembership, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+organizationMembershipColumns+`
		FROM organization_memberships
		WHERE organization_id = $1
		ORDER BY created_at ASC, user_id ASC
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.OrganizationMembership, 0)
	for rows.Next() {
		var m model.OrganizationMembership
		if err := scanOrganizationMembership(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, toDomainOrganizationMembership(m))
	}
	return out, rows.Err()
}

func (s *Store) UpdateOrganizationMembershipRole(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID, role domain.OrganizationRole) (domain.OrganizationMembership, error) {
	var m model.OrganizationMembership

	err := scanOrganizationMembership(s.queryRow(ctx, `
		UPDATE organization_memberships
		SET role = $3
		WHERE organization_id = $1 AND user_id = $2
		RETURNING `+organizationMembershipColumns,
		organizationID,
		userID,
		role,
	), &m)
	if err != nil {
		return domain.OrganizationMembership{}, mapNoRows(err, ErrOrganizationMembershipNotFound)
	}

	return toDomainOrganizationMembership(m), nil
}

func (s *Store) DeleteOrganizationMembership(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID) error {
	res, err := s.exec(ctx, `
		DELETE FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2
	`, organizationID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOrganizationMembershipNotFound
	}
	return nil
}

func (s *Store) ListOrganizationMembershipsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.OrganizationMembership, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+organizationMembershipColumns+`
		FROM organization_memberships
		WHERE user_id = $1
		ORDER BY created_at ASC, organization_id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.OrganizationMembership, 0)
	for rows.Next() {
		var m model.OrganizationMembership
		if err := scanOrganizationMembership(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, toDomainOrganizationMembership(m))
	}
	return out, rows.Err()
}

func (s *Store) GetPersonalOrganizationForUser(ctx context.Context, userID uuid.UUID) (domain.Organization, domain.OrganizationMembership, error) {
	var org model.Organization
	var membership model.OrganizationMembership

	err := s.queryRow(ctx, `
		SELECT
			o.id, o.created_at, o.updated_at, o.name, o.kind, o.created_by_user_id,
			om.organization_id, om.user_id, om.role, om.created_at, om.updated_at
		FROM organizations o
		JOIN organization_memberships om ON om.organization_id = o.id AND om.user_id = $1
		WHERE o.kind = $2 AND o.created_by_user_id = $1
		ORDER BY o.created_at ASC
		LIMIT 1
	`, userID, domain.OrganizationKindPersonal).Scan(
		&org.ID,
		&org.CreatedAt,
		&org.UpdatedAt,
		&org.Name,
		&org.Kind,
		&org.CreatedByUserID,
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	)
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, mapNoRows(err, ErrOrganizationNotFound)
	}

	return toDomainOrganization(org), toDomainOrganizationMembership(membership), nil
}

func (s *Store) EnsureDefaultOrganizationForUser(ctx context.Context, userID uuid.UUID, name string) (domain.Organization, domain.OrganizationMembership, error) {
	return s.EnsureOrganizationForUser(ctx, userID, name, domain.OrganizationKindPersonal)
}

func (s *Store) EnsureOrganizationForUser(ctx context.Context, userID uuid.UUID, name string, kind domain.OrganizationKind) (domain.Organization, domain.OrganizationMembership, error) {
	org, membership, _, err := s.EnsureOrganizationForUserWithCreated(ctx, userID, name, kind)
	return org, membership, err
}

func (s *Store) EnsureOrganizationForUserWithCreated(ctx context.Context, userID uuid.UUID, name string, kind domain.OrganizationKind) (domain.Organization, domain.OrganizationMembership, bool, error) {
	var org model.Organization
	name, err := normalizeOrganizationName(name)
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, false, err
	}

	if kind == domain.OrganizationKindPersonal {
		err := scanOrganization(s.queryRow(ctx, `
			INSERT INTO organizations (name, kind, created_by_user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (created_by_user_id) WHERE kind = 'personal' AND created_by_user_id IS NOT NULL DO NOTHING
			RETURNING `+organizationColumns,
			name,
			kind,
			userID,
		), &org)
		if err != nil && err != sql.ErrNoRows {
			return domain.Organization{}, domain.OrganizationMembership{}, false, err
		}
		if err == nil {
			org, membership, err := s.ensureOwnerMembership(ctx, toDomainOrganization(org), userID)
			return org, membership, true, err
		}
	}

	err = scanOrganization(s.queryRow(ctx, `
			SELECT `+organizationColumns+`
			FROM organizations
			WHERE kind = $1 AND created_by_user_id = $2
			LIMIT 1
		`, kind, userID), &org)
	if err != nil && err != sql.ErrNoRows {
		return domain.Organization{}, domain.OrganizationMembership{}, false, err
	}
	created := false
	if err == sql.ErrNoRows {
		err = scanOrganization(s.queryRow(ctx, `
			INSERT INTO organizations (name, kind, created_by_user_id)
			VALUES ($1, $2, $3)
			RETURNING `+organizationColumns,
			name,
			kind,
			userID,
		), &org)
		if err != nil {
			return domain.Organization{}, domain.OrganizationMembership{}, false, err
		}
		created = true
	}

	orgOut, membership, err := s.ensureOwnerMembership(ctx, toDomainOrganization(org), userID)
	return orgOut, membership, created, err
}

func normalizeOrganizationName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalidOrganizationName
	}
	return name, nil
}

func (s *Store) ensureOwnerMembership(ctx context.Context, org domain.Organization, userID uuid.UUID) (domain.Organization, domain.OrganizationMembership, error) {
	_, err := s.exec(ctx, `
		INSERT INTO organization_memberships (organization_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, user_id) DO NOTHING
	`, org.ID, userID, domain.OrganizationRoleOwner)
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, err
	}

	membership, err := s.GetOrganizationMembership(ctx, org.ID, userID)
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, err
	}

	return org, membership, nil
}
