package admin

import (
	"context"

	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

func (s *Service) SearchUser(ctx context.Context, query string) (UserSummary, error) {
	user, err := s.store.GetUserByEmailOrUsername(ctx, query)
	if err != nil {
		return UserSummary{}, err
	}
	return s.userSummary(ctx, user)
}

func (s *Service) GetUserDetail(ctx context.Context, actor Actor, userID uuid.UUID) (UserDetail, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	summary, err := s.userSummary(ctx, user)
	if err != nil {
		return UserDetail{}, err
	}

	providers, err := s.store.ListAuthProvidersByUserID(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	passkeys, err := s.store.ListPasskeysByUserID(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	sessions, err := s.store.ListSessionsByUserID(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	actions, err := s.userDetailActions(ctx, actor, summary)
	if err != nil {
		return UserDetail{}, err
	}

	now := s.now()
	return UserDetail{
		User:          summary,
		AuthProviders: summarizeAuthProviders(providers),
		Passkeys:      summarizePasskeys(passkeys),
		Sessions:      summarizeSessions(sessions, now),
		Actions:       actions,
	}, nil
}

func (s *Service) DisableUser(ctx context.Context, actor Actor, userID uuid.UUID, meta RequestMeta) error {
	if actor.UserID == userID {
		return ErrSelfDisable
	}

	now := s.now()
	if err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
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

		if err := s.store.DisableUser(txCtx, userID, now); err != nil {
			return err
		}
		if _, err := s.store.RevokeAllActiveSessionsForUser(txCtx, userID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensByUserID(txCtx, userID); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserDisabled, &userID, user.Email, map[string]any{}, meta)
	}); err != nil {
		return err
	}

	_ = s.webhookPublisher.Publish(ctx, webhook.NewUserUpdated(userID, now))
	return nil
}

func (s *Service) EnableUser(ctx context.Context, actor Actor, userID uuid.UUID, meta RequestMeta) error {
	now := s.now()
	if err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		if err := s.store.EnableUser(txCtx, userID); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserEnabled, &userID, user.Email, map[string]any{}, meta)
	}); err != nil {
		return err
	}

	_ = s.webhookPublisher.Publish(ctx, webhook.NewUserUpdated(userID, now))
	return nil
}

func (s *Service) userDetailActions(ctx context.Context, actor Actor, user UserSummary) (UserDetailActions, error) {
	hasAdmin := user.HasRole(roles.DBAdminRoleName)
	activeAdminCount := 0
	var err error
	if hasAdmin && !user.Disabled() {
		activeAdminCount, err = s.store.CountActiveUsersWithRole(ctx, roles.DBAdminRoleName)
		if err != nil {
			return UserDetailActions{}, err
		}
	}

	return UserDetailActions{
		Disable:           s.canDisableUser(actor.UserID, user, activeAdminCount),
		Enable:            ActionAvailability{Allowed: user.Disabled()},
		GrantAdmin:        ActionAvailability{Allowed: !hasAdmin},
		RevokeAdmin:       s.canRevokeAdmin(actor.UserID, user, activeAdminCount),
		RevokeAllSessions: s.CanRevokeAllSessions(actor.UserID, user.ID),
	}, nil
}

func (s *Service) canDisableUser(actorID uuid.UUID, user UserSummary, activeAdminCount int) ActionAvailability {
	if user.Disabled() {
		return ActionAvailability{}
	}
	if actorID == user.ID {
		return ActionAvailability{Reason: ReasonSelfDisable}
	}
	if user.HasRole(roles.DBAdminRoleName) && activeAdminCount <= 1 {
		return ActionAvailability{Reason: ReasonLastAdmin}
	}
	return ActionAvailability{Allowed: true}
}
