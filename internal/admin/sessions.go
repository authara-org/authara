package admin

import (
	"context"

	"github.com/google/uuid"
)

func (s *Service) RevokeUserSession(ctx context.Context, actor Actor, userID, sessionID uuid.UUID, meta RequestMeta) error {
	now := s.now()
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		if err := s.store.RevokeSessionByIDAndUserID(txCtx, sessionID, userID, now); err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensBySession(txCtx, sessionID); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserSessionRevoked, &userID, user.Email, map[string]any{
			"session_id": sessionID.String(),
		}, meta)
	})
}

func (s *Service) RevokeAllUserSessions(ctx context.Context, actor Actor, userID uuid.UUID, meta RequestMeta) error {
	if actor.UserID == userID {
		return ErrSelfRevokeSessions
	}

	now := s.now()
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		revoked, err := s.store.RevokeAllActiveSessionsForUser(txCtx, userID, now)
		if err != nil {
			return err
		}
		if err := s.store.DeleteRefreshTokensByUserID(txCtx, userID); err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionUserSessionsRevoked, &userID, user.Email, map[string]any{
			"revoked_sessions": revoked,
		}, meta)
	})
}

func (s *Service) CanRevokeAllSessions(actorID uuid.UUID, targetID uuid.UUID) ActionAvailability {
	if actorID == targetID {
		return ActionAvailability{Reason: ReasonSelfRevokeSessions}
	}
	return ActionAvailability{Allowed: true}
}
