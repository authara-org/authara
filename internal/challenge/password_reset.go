package challenge

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type CreatePasswordResetChallengeInput struct {
	UserID       uuid.UUID
	Email        string
	PasswordHash string
}

type VerifyPasswordResetChallengeResult struct {
	Challenge domain.Challenge
	Action    domain.PendingPasswordReset
}

func (s *Service) CreatePasswordResetChallenge(
	ctx context.Context,
	in CreatePasswordResetChallengeInput,
	now time.Time,
) (uuid.UUID, error) {
	return s.createChallenge(
		ctx,
		domain.ChallengePurposePasswordReset,
		in.Email,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			_, err := s.store.CreatePendingPasswordReset(txCtx, domain.PendingPasswordReset{
				ChallengeID:  challenge.ID,
				UserID:       in.UserID,
				PasswordHash: in.PasswordHash,
			})
			return err
		},
	)
}

func (s *Service) VerifyPasswordResetChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) (*VerifyPasswordResetChallengeResult, error) {
	challenge, err := s.verifyChallenge(ctx, challengeID, code, verifier, now)
	if err != nil {
		return nil, err
	}

	if challenge.Purpose != domain.ChallengePurposePasswordReset {
		return nil, ErrUnsupportedChallengePurpose
	}

	action, err := s.store.GetPendingPasswordResetByChallengeID(ctx, challenge.ID)
	if err != nil {
		return nil, err
	}

	return &VerifyPasswordResetChallengeResult{
		Challenge: *challenge,
		Action:    action,
	}, nil
}

func (s *Service) ExecutePasswordReset(
	ctx context.Context,
	action domain.PendingPasswordReset,
	now time.Time,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.UpdatePasswordHash(txCtx, action.UserID, action.PasswordHash); err != nil {
			return err
		}
		if err := s.store.RevokeAllSessionsForUser(txCtx, action.UserID, now); err != nil {
			return err
		}
		if err := s.store.DeletePendingPasswordResetByChallengeID(txCtx, action.ChallengeID); err != nil {
			return err
		}
		return nil
	})
}
