package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainAllowedEmail(m model.AllowedEmail) domain.AllowedEmail {
	return domain.AllowedEmail{
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		Email:     m.Email,
	}
}

func toModelAllowedEmail(d domain.AllowedEmail) model.AllowedEmail {
	return model.AllowedEmail{
		Email: d.Email,
	}
}

const allowedEmailColumns = `
	id,
	created_at,
	updated_at,
	email
`

func scanAllowedEmail(row rowScanner, m *model.AllowedEmail) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.Email,
	)
}

func (s *Store) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	var exists bool

	if err := s.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM allowed_emails WHERE email = $1)`, email).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) CreateAllowedEmail(ctx context.Context, allowedEmail domain.AllowedEmail) error {
	m := toModelAllowedEmail(allowedEmail)

	_, err := s.exec(ctx, `INSERT INTO allowed_emails (email) VALUES ($1)`, m.Email)
	if IsUniqueViolation(err, ConstraintAllowedEmailEmail) {
		return ErrAllowedEmailAlreadyExists
	}
	return err
}

func (s *Store) DeleteAllowedEmail(ctx context.Context, email string) error {
	res, err := s.exec(ctx, `DELETE FROM allowed_emails WHERE email = $1`, email)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAllowedEmailNotFound
	}
	return nil
}

func (s *Store) DeleteAllowedEmailByID(ctx context.Context, id uuid.UUID) (domain.AllowedEmail, error) {
	var row model.AllowedEmail
	if err := scanAllowedEmail(s.queryRow(ctx, `
		DELETE FROM allowed_emails
		WHERE id = $1
		RETURNING `+allowedEmailColumns,
		id,
	), &row); err != nil {
		return domain.AllowedEmail{}, mapNoRows(err, ErrAllowedEmailNotFound)
	}
	return toDomainAllowedEmail(row), nil
}

func (s *Store) CountAllowedEmails(ctx context.Context, query string) (int, error) {
	var count int
	query = normalizeEmail(query)
	pattern := "%" + query + "%"

	err := s.queryRow(ctx, `
		SELECT count(*)
		FROM allowed_emails
		WHERE $1 = '' OR lower(email) LIKE $2
	`, query, pattern).Scan(&count)
	return count, err
}

func (s *Store) ListAllowedEmails(ctx context.Context) ([]domain.AllowedEmail, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+allowedEmailColumns+`
		FROM allowed_emails
		ORDER BY email ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AllowedEmail, 0)
	for rows.Next() {
		var row model.AllowedEmail
		if err := scanAllowedEmail(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainAllowedEmail(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) ListAllowedEmailsPage(ctx context.Context, query string, limit, offset int) ([]domain.AllowedEmail, error) {
	query = normalizeEmail(query)
	pattern := "%" + query + "%"

	rows, err := s.queryRows(ctx, `
		SELECT `+allowedEmailColumns+`
		FROM allowed_emails
		WHERE $1 = '' OR lower(email) LIKE $2
		ORDER BY email ASC
		LIMIT $3 OFFSET $4
	`, query, pattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AllowedEmail, 0)
	for rows.Next() {
		var row model.AllowedEmail
		if err := scanAllowedEmail(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainAllowedEmail(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
