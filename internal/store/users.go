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

func (s *Store) GetUserByEmailOrUsername(ctx context.Context, query string) (domain.User, error) {
	var m model.User

	email := normalizeEmail(query)
	username := NormalizeUsername(query)
	usernameExact := strings.TrimSpace(query)

	err := scanUser(s.queryRow(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE email = $1 OR username_normalized = $2
		ORDER BY
			CASE
				WHEN email = $1 THEN 0
				WHEN username = $3 THEN 1
				WHEN username_normalized = $2 THEN 2
				ELSE 3
			END,
			created_at DESC
		LIMIT 1
	`, email, username, usernameExact), &m)
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
	res, err := s.exec(ctx, `UPDATE users SET disabled_at = $1 WHERE id = $2`, disabledAt, userID)
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

func (s *Store) EnableUser(ctx context.Context, userID uuid.UUID) error {
	res, err := s.exec(ctx, `UPDATE users SET disabled_at = NULL WHERE id = $1`, userID)
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

func (s *Store) IsUserDisabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool

	err := s.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND disabled_at IS NOT NULL)`, userID).
		Scan(&exists)
	return exists, err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.queryRow(ctx, `SELECT count(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) CountUsersCreatedSince(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := s.queryRow(ctx, `SELECT count(*) FROM users WHERE created_at >= $1`, since).Scan(&count)
	return count, err
}

func (s *Store) CountDisabledUsers(ctx context.Context) (int, error) {
	var count int
	err := s.queryRow(ctx, `SELECT count(*) FROM users WHERE disabled_at IS NOT NULL`).Scan(&count)
	return count, err
}

func (s *Store) CountUsersWithRole(ctx context.Context, roleName string) (int, error) {
	var count int
	err := s.queryRow(ctx, `
		SELECT count(*)
		FROM users u
		JOIN user_platform_roles upr ON upr.user_id = u.id
		JOIN platform_roles pr ON pr.id = upr.role_id
		WHERE pr.name = $1
	`, roleName).Scan(&count)
	return count, err
}

func (s *Store) CountActiveUsersWithRole(ctx context.Context, roleName string) (int, error) {
	var count int
	err := s.queryRow(ctx, `
		SELECT count(*)
		FROM users u
		JOIN user_platform_roles upr ON upr.user_id = u.id
		JOIN platform_roles pr ON pr.id = upr.role_id
		WHERE pr.name = $1 AND u.disabled_at IS NULL
	`, roleName).Scan(&count)
	return count, err
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
