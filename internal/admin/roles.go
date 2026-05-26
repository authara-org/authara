package admin

import (
	"context"

	"github.com/authara-org/authara/internal/session/roles"
	"github.com/google/uuid"
)

func (s *Service) GrantAdmin(ctx context.Context, actor Actor, userID uuid.UUID, meta RequestMeta) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		if err := s.store.AddUserPlatformRoleByName(txCtx, userID, roles.DBAdminRoleName); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserAdminGranted, &userID, user.Email, map[string]any{}, meta)
	})
}

func (s *Service) RevokeAdmin(ctx context.Context, actor Actor, userID uuid.UUID, meta RequestMeta) error {
	if actor.UserID == userID {
		return ErrSelfRevokeAdmin
	}

	now := s.now()
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.LockPlatformRoleByName(txCtx, roles.DBAdminRoleName); err != nil {
			return err
		}

		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		hasAdmin, err := s.store.UserHasPlatformRole(txCtx, userID, roles.DBAdminRoleName)
		if err != nil {
			return err
		}
		if hasAdmin && user.DisabledAt == nil {
			activeAdmins, err := s.store.CountActiveUsersWithRole(txCtx, roles.DBAdminRoleName)
			if err != nil {
				return err
			}
			if activeAdmins <= 1 {
				return ErrLastAdmin
			}
		}

		if err := s.store.RemoveUserPlatformRoleByName(txCtx, userID, roles.DBAdminRoleName); err != nil {
			return err
		}
		if _, err := s.store.RevokeAllActiveSessionsForUser(txCtx, userID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensByUserID(txCtx, userID); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserAdminRevoked, &userID, user.Email, map[string]any{}, meta)
	})
}

func (s *Service) canRevokeAdmin(actorID uuid.UUID, user UserSummary, activeAdminCount int) ActionAvailability {
	if !user.HasRole(roles.DBAdminRoleName) {
		return ActionAvailability{}
	}
	if actorID == user.ID {
		return ActionAvailability{Reason: ReasonSelfRevokeAdmin}
	}
	if !user.Disabled() && activeAdminCount <= 1 {
		return ActionAvailability{Reason: ReasonLastAdmin}
	}
	return ActionAvailability{Allowed: true}
}
