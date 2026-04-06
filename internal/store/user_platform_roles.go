package store

import (
	"context"

	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func (s *Store) GetUserPlatformRoleNames(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var names []string

	err := s.dbFromContext(ctx).
		Table("user_platform_roles upr").
		Select("pr.name").
		Joins("JOIN platform_roles pr ON pr.id = upr.role_id").
		Where("upr.user_id = ?", userID).
		Order("pr.name ASC").
		Scan(&names).Error
	if err != nil {
		return nil, err
	}

	return names, nil
}

func (s *Store) AddUserPlatformRoleByName(ctx context.Context, userID uuid.UUID, roleName string) error {
	roleID, err := s.getPlatformRoleIDByName(ctx, roleName)
	if err != nil {
		return err
	}

	return s.dbFromContext(ctx).
		Exec(`
			INSERT INTO user_platform_roles (user_id, role_id)
			VALUES (?, ?)
			ON CONFLICT (user_id, role_id) DO NOTHING
		`, userID, roleID).
		Error
}

func (s *Store) RemoveUserPlatformRoleByName(ctx context.Context, userID uuid.UUID, roleName string) error {
	roleID, err := s.getPlatformRoleIDByName(ctx, roleName)
	if err != nil {
		return err
	}

	return s.dbFromContext(ctx).
		Exec(`
			DELETE FROM user_platform_roles
			WHERE user_id = ? AND role_id = ?
		`, userID, roleID).
		Error
}

// ---- helpers ----

func (s *Store) getPlatformRoleIDByName(ctx context.Context, name string) (uuid.UUID, error) {
	var role model.Role

	err := s.dbFromContext(ctx).
		Where("name = ?", name).
		First(&role).Error
	if err != nil {
		return uuid.Nil, err
	}

	if role.ID == nil {
		return uuid.Nil, ErrorRoleNotFound
	}

	return *role.ID, nil
}
