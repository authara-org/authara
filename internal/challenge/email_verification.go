package challenge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

func (s *Service) BeginRequiredEmailVerification(
	ctx context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	audience string,
	returnTo string,
	now time.Time,
) (domain.EmailVerificationTransaction, error) {
	var transaction domain.EmailVerificationTransaction
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user, err := s.store.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		if user.EmailVerifiedAt != nil {
			return nil
		}
		session, err := s.store.GetActiveSessionByIDForUpdate(txCtx, sessionID, now)
		if err != nil || session.UserID != userID {
			return ErrEmailVerificationInvalid
		}
		policy := s.policy.CurrentChallenge()
		transaction, err = s.store.UpsertEmailVerificationTransaction(txCtx, domain.EmailVerificationTransaction{
			UserID:               userID,
			OriginalSessionID:    sessionID,
			Audience:             audience,
			AuthenticationMethod: session.AuthenticationMethod,
			ReturnTo:             returnTo,
			ExpiresAt:            now.Add(policy.TTL),
		})
		if err != nil {
			return err
		}
		if s.accessTokenRevocations != nil {
			if err := s.accessTokenRevocations.RevokeSession(txCtx, sessionID, now); err != nil {
				return err
			}
		}
		return s.store.RevokeSession(txCtx, sessionID, now)
	})
	return transaction, err
}

func (s *Service) GetEmailVerificationTransaction(ctx context.Context, id uuid.UUID, now time.Time) (domain.EmailVerificationTransaction, domain.User, error) {
	transaction, err := s.store.GetEmailVerificationTransactionByID(ctx, id)
	if err != nil || !transaction.IsActive(now) {
		return domain.EmailVerificationTransaction{}, domain.User{}, ErrEmailVerificationInvalid
	}
	user, err := s.store.GetUserByID(ctx, transaction.UserID)
	if err != nil || user.EmailVerifiedAt != nil {
		return domain.EmailVerificationTransaction{}, domain.User{}, ErrEmailVerificationInvalid
	}
	return transaction, user, nil
}

func (s *Service) CreateRequiredEmailVerificationChallenge(
	ctx context.Context,
	transactionID uuid.UUID,
	targetEmail string,
	now time.Time,
) (uuid.UUID, error) {
	targetEmail = strings.ToLower(strings.TrimSpace(targetEmail))
	return s.createChallenge(ctx, domain.ChallengePurposeEmailVerification, targetEmail, now, func(txCtx context.Context, challenge domain.Challenge) error {
		transaction, err := s.store.GetEmailVerificationTransactionByIDForUpdate(txCtx, transactionID)
		if err != nil || !transaction.IsActive(now) {
			return ErrEmailVerificationInvalid
		}
		user, err := s.store.GetUserByIDForUpdate(txCtx, transaction.UserID)
		if err != nil || user.EmailVerifiedAt != nil {
			return ErrEmailVerificationInvalid
		}
		if !strings.EqualFold(user.Email, targetEmail) {
			exists, err := s.store.UserExistsByEmail(txCtx, targetEmail)
			if err != nil {
				return err
			}
			if exists {
				return ErrEmailAlreadyInUse
			}
		}
		return s.store.AttachEmailVerificationChallenge(txCtx, transaction.ID, challenge.ID, targetEmail)
	})
}

func (s *Service) CompleteRequiredEmailVerification(
	ctx context.Context,
	transactionID uuid.UUID,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) (domain.EmailVerificationTransaction, domain.User, error) {
	var transaction domain.EmailVerificationTransaction
	var user domain.User
	_, err := s.verifyChallenge(ctx, challengeID, domain.ChallengePurposeEmailVerification, code, verifier, now,
		func(txCtx context.Context, _ domain.Challenge) error {
			var err error
			transaction, err = s.store.GetEmailVerificationTransactionByChallengeIDForUpdate(txCtx, challengeID)
			if err != nil || transaction.ID != transactionID || !transaction.IsActive(now) || transaction.TargetEmail == nil {
				return ErrEmailVerificationInvalid
			}
			user, err = s.store.GetUserByIDForUpdate(txCtx, transaction.UserID)
			if err != nil || user.EmailVerifiedAt != nil {
				return ErrEmailVerificationInvalid
			}
			return nil
		},
		func(txCtx context.Context, _ domain.Challenge) error {
			targetEmail := *transaction.TargetEmail
			if strings.EqualFold(user.Email, targetEmail) {
				updated, err := s.store.MarkUserEmailVerified(txCtx, user.ID, user.Email, now)
				if err != nil || !updated {
					return ErrEmailVerificationInvalid
				}
			} else {
				updated, err := s.store.UpdateUserEmailVerifiedIfCurrent(txCtx, user.ID, user.Email, targetEmail, now)
				if err != nil {
					if store.IsUniqueViolation(err, store.ConstraintUserEmail) {
						return ErrEmailAlreadyInUse
					}
					return err
				}
				if !updated {
					return ErrEmailVerificationInvalid
				}
				if s.allowlistPolicy.CurrentAllowlist().AllowlistEnabled {
					if err := s.store.DeleteAllowedEmail(txCtx, user.Email); err != nil && !errors.Is(err, store.ErrAllowedEmailNotFound) {
						return err
					}
					if err := s.store.EnsureAllowedEmail(txCtx, targetEmail); err != nil {
						return err
					}
				}
				user.Email = targetEmail
			}
			user.EmailVerifiedAt = &now
			if err := s.store.ConsumeEmailVerificationTransaction(txCtx, transaction.ID, now); err != nil {
				return err
			}
			if err := s.webhookPublisher.Publish(txCtx, webhook.NewUserUpdated(user.ID, now)); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		return domain.EmailVerificationTransaction{}, domain.User{}, err
	}
	return transaction, user, nil
}
