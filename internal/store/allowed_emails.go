package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
)

func toDomainAllowedEmail(m model.AllowedEmail) domain.AllowedEmail {
	return domain.AllowedEmail{
		ID:    m.ID,
		Email: m.Email,
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
	return err
}

func (s *Store) DeleteAllowedEmail(ctx context.Context, email string) error {
	_, err := s.exec(ctx, `DELETE FROM allowed_emails WHERE email = $1`, email)
	return err
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
