package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainAuthProvider(m model.AuthProvider) domain.AuthProvider {
	return domain.AuthProvider{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		UserID:   m.UserID,
		Provider: domain.Provider(m.Provider),

		ProviderUserID: m.ProviderUserID,
		PasswordHash:   m.PasswordHash,
	}
}

func toModelAuthProvider(d domain.AuthProvider) model.AuthProvider {
	return model.AuthProvider{
		UserID:   d.UserID,
		Provider: string(d.Provider),

		ProviderUserID: d.ProviderUserID,
		PasswordHash:   d.PasswordHash,
	}
}

const authProviderColumns = `
	id,
	user_id,
	provider,
	provider_user_id,
	password_hash,
	created_at,
	updated_at
`

func scanAuthProvider(row rowScanner, m *model.AuthProvider) error {
	return row.Scan(
		&m.ID,
		&m.UserID,
		&m.Provider,
		&m.ProviderUserID,
		&m.PasswordHash,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
}

func (s *Store) ListAuthProvidersByUserID(ctx context.Context, userID uuid.UUID) ([]domain.AuthProvider, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+authProviderColumns+`
		FROM auth_providers
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AuthProvider, 0)
	for rows.Next() {
		var row model.AuthProvider
		if err := scanAuthProvider(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainAuthProvider(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) CreateAuthProvider(ctx context.Context, provider domain.AuthProvider) (domain.AuthProvider, error) {
	m := toModelAuthProvider(provider)

	if err := scanAuthProvider(s.queryRow(ctx, `
		INSERT INTO auth_providers (user_id, provider, provider_user_id, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING `+authProviderColumns,
		m.UserID,
		m.Provider,
		m.ProviderUserID,
		m.PasswordHash,
	), &m); err != nil {
		return domain.AuthProvider{}, err
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) GetAuthProviderByMethodAndUserID(ctx context.Context, provider domain.Provider, userID uuid.UUID) (domain.AuthProvider, error) {
	var m model.AuthProvider

	err := scanAuthProvider(s.queryRow(ctx, `
		SELECT `+authProviderColumns+`
		FROM auth_providers
		WHERE user_id = $1 AND provider = $2
	`, userID, string(provider)), &m)
	if err != nil {
		return domain.AuthProvider{}, mapNoRows(err, ErrorAuthProviderNotFound)
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) GetAuthProviderByProviderAndProviderUserID(ctx context.Context, provider domain.Provider, providerUserID string) (domain.AuthProvider, error) {
	var m model.AuthProvider

	err := scanAuthProvider(s.queryRow(ctx, `
		SELECT `+authProviderColumns+`
		FROM auth_providers
		WHERE provider_user_id = $1 AND provider = $2
	`, providerUserID, string(provider)), &m)
	if err != nil {
		return domain.AuthProvider{}, mapNoRows(err, ErrorAuthProviderNotFound)
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) DeleteAuthProviderByMethodAndUserID(ctx context.Context, provider domain.Provider, userID uuid.UUID) error {
	res, err := s.exec(ctx, `
		DELETE FROM auth_providers
		WHERE user_id = $1 AND provider = $2
	`, userID, string(provider))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrorAuthProviderNotFound
	}

	return nil
}

func (s *Store) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	res, err := s.exec(ctx, `
		UPDATE auth_providers
		SET password_hash = $1
		WHERE user_id = $2 AND provider = $3
	`, passwordHash, userID, string(domain.ProviderPassword))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrorAuthProviderNotFound
	}

	return nil
}
