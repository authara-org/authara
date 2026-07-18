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
	InvitationID *uuid.UUID
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
				InvitationID: in.InvitationID,
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
	complete func(context.Context, domain.PendingSignupAction) error,
) (*VerifySignupChallengeResult, error) {
	var action domain.PendingSignupAction
	challenge, err := s.verifyChallenge(
		ctx,
		challengeID,
		domain.ChallengePurposeSignup,
		code,
		verifier,
		now,
		func(txCtx context.Context, challenge domain.Challenge) error {
			var err error
			action, err = s.store.GetPendingSignupActionByChallengeID(txCtx, challenge.ID)
			if err != nil {
				return err
			}
			if complete != nil {
				return complete(txCtx, action)
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return &VerifySignupChallengeResult{
		Challenge: *challenge,
		Action:    action,
	}, nil
}
