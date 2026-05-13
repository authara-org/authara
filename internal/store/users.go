package store

import (
	"context"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func NormalizeUsername(u string) string {
	return strings.ToLower(strings.TrimSpace(u))
}

func toDomainUser(m model.User) domain.User {
	return domain.User{
		ID:         m.ID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		DisabledAt: m.DisabledAt,
		Username:   m.Username,
		Email:      m.Email,
	}
}

func toModelUser(d domain.User) model.User {
	return model.User{
		Username:           d.Username,
		UsernameNormalized: NormalizeUsername(d.Username),
		Email:              d.Email,
		DisabledAt:         d.DisabledAt,
	}
}

const userColumns = `
	id,
	created_at,
	updated_at,
	disabled_at,
	username,
	username_normalized,
	email
`

func scanUser(row rowScanner, m *model.User) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DisabledAt,
		&m.Username,
		&m.UsernameNormalized,
		&m.Email,
	)
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	m := toModelUser(user)
	m.UsernameNormalized = NormalizeUsername(user.Username)

	if err := scanUser(s.queryRow(ctx, `
		INSERT INTO users (username, username_normalized, email, disabled_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+userColumns,
		m.Username,
		m.UsernameNormalized,
		m.Email,
		m.DisabledAt,
	), &m); err != nil {
		return domain.User{}, err
	}

	return toDomainUser(m), nil
}

func (s *Store) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	var m model.User

	err := scanUser(s.queryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, userID), &m)
	if err != nil {
		return domain.User{}, mapNoRows(err, ErrUserNotFound)
	}

	return toDomainUser(m), nil
}

func (s *Store) LockUserForAuthMethodMutation(ctx context.Context, userID uuid.UUID) error {
	var lockedID uuid.UUID
	err := s.queryRow(ctx, `
		SELECT id
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&lockedID)
	return mapNoRows(err, ErrUserNotFound)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var m model.User

	email = normalizeEmail(email)

	err := scanUser(s.queryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email), &m)
	if err != nil {
		return domain.User{}, mapNoRows(err, ErrUserNotFound)
	}

	return toDomainUser(m), nil
}

func (s *Store) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool

	email = normalizeEmail(email)

	if err := s.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (s *Store) DisableUser(ctx context.Context, userID uuid.UUID, disabledAt time.Time) error {
	_, err := s.exec(ctx, `UPDATE users SET disabled_at = $1 WHERE id = $2`, disabledAt, userID)
	return err
}

func (s *Store) IsUserDisabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool

	err := s.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND disabled_at IS NOT NULL)`, userID).
		Scan(&exists)
	return exists, err
}

func (s *Store) UpdateUsername(ctx context.Context, userID uuid.UUID, username string) error {
	res, err := s.exec(ctx, `
		UPDATE users
		SET username = $1,
		    username_normalized = $2
		WHERE id = $3
	`, username, NormalizeUsername(username), userID)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s *Store) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}
