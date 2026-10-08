package admin

import (
	"context"

	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func (s *Service) RevokeUserSession(ctx context.Context, actor Actor, userID, sessionID uuid.UUID, meta RequestMeta) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		session, err := s.store.GetSessionByID(txCtx, sessionID)
		if err != nil {
			return err
		}
		if session.UserID != userID {
			return store.ErrSessionNotFound
		}
		now := s.now()
		if err := s.accessTokenRevocations.RevokeSession(txCtx, sessionID, now); err != nil {
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

	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		now := s.now()
		if err := s.accessTokenRevocations.RevokeUser(txCtx, userID, now); err != nil {
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
