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
	"github.com/google/uuid"
)

type CreatePasswordResetChallengeInput struct {
	UserID       uuid.UUID
	Email        string
	PasswordHash string
}

func (s *Service) CreatePasswordResetChallenge(
	ctx context.Context,
	in CreatePasswordResetChallengeInput,
	now time.Time,
) (uuid.UUID, error) {
	challengeID, err := s.createChallenge(
		ctx,
		domain.ChallengePurposePasswordReset,
		in.Email,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			if err := s.store.LockUserForAuthMethodMutation(txCtx, in.UserID); err != nil {
				if errors.Is(err, store.ErrUserNotFound) {
					return ErrPasswordResetUnavailable
				}
				return err
			}
			if _, err := s.requirePasswordResetEligibility(txCtx, in.UserID, in.Email); err != nil {
				return err
			}
			_, err := s.store.CreatePendingPasswordReset(txCtx, domain.PendingPasswordReset{
				ChallengeID:  challenge.ID,
				UserID:       in.UserID,
				PasswordHash: in.PasswordHash,
			})
			return err
		},
	)
	if errors.Is(err, ErrPasswordResetUnavailable) {
		// Keep the public recovery response indistinguishable from an unknown
		// address without issuing a usable password-reset code.
		return s.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposePasswordReset, in.Email)
	}
	return challengeID, err
}

func (s *Service) CompletePasswordResetChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) error {
	_, err := s.verifyChallenge(
		ctx,
		challengeID,
		domain.ChallengePurposePasswordReset,
		code,
		verifier,
		now,
		nil,
		func(txCtx context.Context, challenge domain.Challenge) error {
			action, err := s.store.GetPendingPasswordResetByChallengeID(txCtx, challenge.ID)
			if errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
				return ErrPasswordResetUnavailable
			}
			if err != nil {
				return err
			}
			if err := s.store.LockUserForAuthMethodMutation(txCtx, action.UserID); err != nil {
				if errors.Is(err, store.ErrUserNotFound) {
					return ErrPasswordResetUnavailable
				}
				return err
			}

			// Re-read after acquiring the user lock. An authenticated password
			// replacement may have cancelled this reset while we were waiting.
			action, err = s.store.GetPendingPasswordResetByChallengeID(txCtx, challenge.ID)
			if errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
				return ErrPasswordResetUnavailable
			}
			if err != nil {
				return err
			}
			user, err := s.requirePasswordResetEligibility(txCtx, action.UserID, challenge.Email)
			if err != nil {
				return err
			}
			markerAt := time.Now().UTC()
			if now.After(markerAt) {
				markerAt = now
			}
			return s.executePasswordReset(txCtx, user, action, now, markerAt)
		},
	)
	return err
}

func (s *Service) requirePasswordResetEligibility(
	ctx context.Context,
	userID uuid.UUID,
	challengeEmail string,
) (domain.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if errors.Is(err, store.ErrUserNotFound) {
		return domain.User{}, ErrPasswordResetUnavailable
	}
	if err != nil {
		return domain.User{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(challengeEmail)) {
		return domain.User{}, ErrPasswordResetUnavailable
	}
	if s.authenticationPolicy.CurrentAuthentication().EmailVerificationRequired && user.EmailVerifiedAt == nil {
		return domain.User{}, ErrPasswordResetUnavailable
	}

	provider, err := s.store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, userID)
	if errors.Is(err, store.ErrorAuthProviderNotFound) {
		return domain.User{}, ErrPasswordResetUnavailable
	}
	if err != nil {
		return domain.User{}, err
	}
	if provider.PasswordHash == nil || strings.TrimSpace(*provider.PasswordHash) == "" {
		return domain.User{}, ErrPasswordResetUnavailable
	}
	return user, nil
}

func (s *Service) executePasswordReset(
	ctx context.Context,
	user domain.User,
	action domain.PendingPasswordReset,
	now time.Time,
	markerAt time.Time,
) error {
	if err := s.store.UpdatePasswordHash(ctx, action.UserID, action.PasswordHash); err != nil {
		if errors.Is(err, store.ErrorAuthProviderNotFound) {
			return ErrPasswordResetUnavailable
		}
		return err
	}
	if err := s.accessTokenRevocations.RevokeUser(ctx, action.UserID, markerAt); err != nil {
		return err
	}
	if err := s.store.RevokeAllSessionsForUser(ctx, action.UserID, now); err != nil {
		return err
	}
	if err := s.store.DeletePendingPasswordResetsByUserID(ctx, action.UserID); err != nil {
		return err
	}
	if err := email.Enqueue(ctx, s.store, user.Email, domain.EmailTemplatePasswordChanged, email.TemplateData{
		email.TemplateVariableOccurredAt: email.OccurredAt(now),
	}, now); err != nil {
		return err
	}
	return s.securityEvents.CredentialPasswordReset(ctx, securityevent.Credential{
		ActorType: domain.SecurityEventActorAnonymous,
		UserID:    action.UserID,
	})
}
