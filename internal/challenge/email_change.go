package challenge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type CreateEmailChangeChallengeInput struct {
	UserID              uuid.UUID
	InitiatingSessionID uuid.UUID
	OldEmail            string
	NewEmail            string
}

type CompleteEmailChangeChallengeInput struct {
	ChallengeID uuid.UUID
	UserID      uuid.UUID
	SessionID   uuid.UUID
	Code        string
}

func (s *Service) CreateEmailChangeChallenge(
	ctx context.Context,
	in CreateEmailChangeChallengeInput,
	now time.Time,
) (uuid.UUID, error) {
	return s.createChallenge(
		ctx,
		domain.ChallengePurposeEmailChange,
		in.NewEmail,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			user, err := s.requireActiveEmailChangeSession(txCtx, in.UserID, in.InitiatingSessionID, now)
			if err != nil {
				return err
			}
			if !sameEmail(user.Email, in.OldEmail) {
				return ErrEmailChangeNotAuthorized
			}
			_, err = s.store.CreatePendingEmailChange(txCtx, domain.PendingEmailChange{
				ChallengeID:         challenge.ID,
				UserID:              in.UserID,
				InitiatingSessionID: in.InitiatingSessionID,
				OldEmail:            user.Email,
				NewEmail:            in.NewEmail,
			})
			return err
		},
	)
}

func (s *Service) CompleteEmailChangeChallenge(
	ctx context.Context,
	in CompleteEmailChangeChallengeInput,
	verifier *VerificationCodeService,
	now time.Time,
) error {
	var action domain.PendingEmailChange
	_, err := s.verifyChallenge(
		ctx,
		in.ChallengeID,
		domain.ChallengePurposeEmailChange,
		in.Code,
		verifier,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			user, err := s.requireActiveEmailChangeSession(txCtx, in.UserID, in.SessionID, now)
			if err != nil {
				return err
			}

			action, err = s.store.GetPendingEmailChangeByChallengeIDForUpdate(txCtx, challenge.ID)
			if err != nil {
				if errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
					return ErrEmailChangeNotAuthorized
				}
				return err
			}
			if action.UserID != in.UserID || action.InitiatingSessionID != in.SessionID || !sameEmail(user.Email, action.OldEmail) {
				return ErrEmailChangeNotAuthorized
			}
			return nil
		},
		func(txCtx context.Context, _ domain.Challenge) error {
			return s.executeEmailChange(txCtx, action, now)
		},
	)
	return err
}

func (s *Service) requireActiveEmailChangeSession(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	now time.Time,
) (domain.User, error) {
	user, err := s.store.GetUserByIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return domain.User{}, ErrEmailChangeNotAuthorized
		}
		return domain.User{}, err
	}
	if user.DisabledAt != nil {
		return domain.User{}, ErrEmailChangeNotAuthorized
	}

	session, err := s.store.GetActiveSessionByIDForUpdate(ctx, sessionID, now)
	if err != nil {
		if errors.Is(err, store.ErrSessionNotFound) {
			return domain.User{}, ErrEmailChangeNotAuthorized
		}
		return domain.User{}, err
	}
	if session.UserID != userID {
		return domain.User{}, ErrEmailChangeNotAuthorized
	}
	return user, nil
}

func sameEmail(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func (s *Service) executeEmailChange(
	ctx context.Context,
	action domain.PendingEmailChange,
	now time.Time,
) error {
	if err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		updated, err := s.store.UpdateUserEmailIfCurrent(txCtx, action.UserID, action.OldEmail, action.NewEmail)
		if err != nil {
			if store.IsUniqueViolation(err, store.ConstraintUserEmail) {
				return ErrEmailAlreadyInUse
			}
			return err
		}
		if !updated {
			return ErrEmailChangeNotAuthorized
		}
		if s.allowlistPolicy.CurrentAllowlist().AllowlistEnabled {
			if err := s.store.DeleteAllowedEmail(txCtx, action.OldEmail); err != nil {
				return err
			}
			if err := s.store.EnsureAllowedEmail(txCtx, action.NewEmail); err != nil {
				return err
			}
		}
		if err := s.store.DeletePendingEmailChangesByUserID(txCtx, action.UserID); err != nil {
			return err
		}
		if err := s.store.DeletePendingPasswordResetsByUserID(txCtx, action.UserID); err != nil {
			return err
		}
		data := email.TemplateData{
			email.TemplateVariableOldEmail:   action.OldEmail,
			email.TemplateVariableNewEmail:   action.NewEmail,
			email.TemplateVariableOccurredAt: email.OccurredAt(now),
		}
		if err := email.Enqueue(txCtx, s.store, action.OldEmail, domain.EmailTemplateEmailChangedOldAddress, data, now); err != nil {
			return err
		}
		if err := email.Enqueue(txCtx, s.store, action.NewEmail, domain.EmailTemplateEmailChangedNewAddress, data, now); err != nil {
			return err
		}
		if err := s.webhookPublisher.Publish(txCtx, webhook.NewUserUpdated(action.UserID, now)); err != nil {
			return err
		}
		return s.securityEvents.AccountEmailChanged(txCtx, securityevent.Credential{
			UserID: action.UserID, SessionID: &action.InitiatingSessionID,
		})
	}); err != nil {
		return err
	}
	return nil
}
