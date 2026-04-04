package challenge

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type CreateSignupChallengeInput struct {
	Email        string
	Username     string
	PasswordHash string
}

type VerifySignupChallengeResult struct {
	Challenge domain.Challenge
	Action    domain.PendingSignupAction
}

func (s *Service) CreateSignupChallenge(
	ctx context.Context,
	in CreateSignupChallengeInput,
	now time.Time,
) (uuid.UUID, error) {
	return s.createChallenge(
		ctx,
		domain.ChallengePurposeSignup,
		in.Email,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			_, err := s.store.CreatePendingSignupAction(txCtx, domain.PendingSignupAction{
				ChallengeID:  challenge.ID,
				Email:        in.Email,
				Username:     in.Username,
				PasswordHash: in.PasswordHash,
			})
			return err
		},
	)
}

func (s *Service) VerifySignupChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) (*VerifySignupChallengeResult, error) {
	challenge, err := s.verifyChallenge(ctx, challengeID, code, verifier, now)
	if err != nil {
		return nil, err
	}

	if challenge.Purpose != domain.ChallengePurposeSignup {
		return nil, ErrUnsupportedChallengePurpose
	}

	action, err := s.store.GetPendingSignupActionByChallengeID(ctx, challenge.ID)
	if err != nil {
		return nil, err
	}

	return &VerifySignupChallengeResult{
		Challenge: *challenge,
		Action:    action,
	}, nil
}
