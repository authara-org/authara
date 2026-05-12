package store

import (
	"context"

	"github.com/google/uuid"
)

func (s *Store) GetUserPlatformRoleNames(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := s.queryRows(ctx, `
		SELECT pr.name
		FROM user_platform_roles upr
		JOIN platform_roles pr ON pr.id = upr.role_id
		WHERE upr.user_id = $1
		ORDER BY pr.name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return names, nil
}

func (s *Store) AddUserPlatformRoleByName(ctx context.Context, userID uuid.UUID, roleName string) error {
	roleID, err := s.getPlatformRoleIDByName(ctx, roleName)
	if err != nil {
		return err
	}

	_, err = s.exec(ctx, `
		INSERT INTO user_platform_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, userID, roleID)
	return err
}

func (s *Store) RemoveUserPlatformRoleByName(ctx context.Context, userID uuid.UUID, roleName string) error {
	roleID, err := s.getPlatformRoleIDByName(ctx, roleName)
	if err != nil {
		return err
	}

	_, err = s.exec(ctx, `
		DELETE FROM user_platform_roles
		WHERE user_id = $1 AND role_id = $2
	`, userID, roleID)
	return err
}

// ---- helpers ----

func (s *Store) getPlatformRoleIDByName(ctx context.Context, name string) (uuid.UUID, error) {
	var id uuid.UUID

	err := s.queryRow(ctx, `SELECT id FROM platform_roles WHERE name = $1`, name).Scan(&id)
	if err != nil {
		return uuid.Nil, mapNoRows(err, ErrorRoleNotFound)
	}

	return id, nil
}
