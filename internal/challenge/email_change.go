package challenge

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type CreateEmailChangeChallengeInput struct {
	UserID   uuid.UUID
	OldEmail string
	NewEmail string
}

type VerifyEmailChangeChallengeResult struct {
	Challenge domain.Challenge
	Action    domain.PendingEmailChange
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
			_, err := s.store.CreatePendingEmailChange(txCtx, domain.PendingEmailChange{
				ChallengeID: challenge.ID,
				UserID:      in.UserID,
				OldEmail:    in.OldEmail,
				NewEmail:    in.NewEmail,
			})
			return err
		},
	)
}

func (s *Service) VerifyEmailChangeChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) (*VerifyEmailChangeChallengeResult, error) {
	challenge, err := s.verifyChallenge(ctx, challengeID, code, verifier, now)
	if err != nil {
		return nil, err
	}

	if challenge.Purpose != domain.ChallengePurposeEmailChange {
		return nil, ErrUnsupportedChallengePurpose
	}

	action, err := s.store.GetPendingEmailChangeByChallengeID(ctx, challenge.ID)
	if err != nil {
		return nil, err
	}

	return &VerifyEmailChangeChallengeResult{
		Challenge: *challenge,
		Action:    action,
	}, nil
}

func (s *Service) ExecuteEmailChange(
	ctx context.Context,
	action domain.PendingEmailChange,
	now time.Time,
) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.UpdateUserEmail(txCtx, action.UserID, action.NewEmail); err != nil {
			return err
		}
		if err := s.store.DeletePendingEmailChangeByChallengeID(txCtx, action.ChallengeID); err != nil {
			return err
		}
		return nil
	})
}
