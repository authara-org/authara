package challenge

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/google/uuid"
)

type Config struct {
	Store             *store.Store
	Tx                *tx.Manager
	ChallengeTTL      time.Duration
	MaxAttempts       int
	MaxResends        int
	MinResendInterval time.Duration
}

type Service struct {
	store             *store.Store
	tx                *tx.Manager
	challengeTTL      time.Duration
	maxAttempts       int
	maxResends        int
	minResendInterval time.Duration
}

func New(cfg Config) *Service {
	return &Service{
		store:             cfg.Store,
		tx:                cfg.Tx,
		challengeTTL:      cfg.ChallengeTTL,
		maxAttempts:       cfg.MaxAttempts,
		maxResends:        cfg.MaxResends,
		minResendInterval: cfg.MinResendInterval,
	}
}

func (s *Service) CreateOpaqueChallenge(
	ctx context.Context,
	now time.Time,
	purpose domain.ChallengePurpose,
	email string,
) (uuid.UUID, error) {
	var challengeID uuid.UUID

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, err := s.store.CreateChallenge(txCtx, domain.Challenge{
			Purpose:      purpose,
			Email:        email,
			ExpiresAt:    now.Add(s.challengeTTL),
			AttemptCount: 0,
			MaxAttempts:  s.maxAttempts,
			ResendCount:  0,
			MaxResends:   s.maxResends,
		})

		challengeID = challenge.ID

		return err
	})
	if err != nil {
		return uuid.Nil, err
	}

	return challengeID, nil
}

func (s *Service) createChallenge(
	ctx context.Context,
	purpose domain.ChallengePurpose,
	email string,
	now time.Time,
	createPendingAction func(context.Context, domain.Challenge) error,
) (uuid.UUID, error) {
	var challengeID uuid.UUID

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, err := s.store.CreateChallenge(txCtx, domain.Challenge{
			Purpose:      purpose,
			Email:        email,
			ExpiresAt:    now.Add(s.challengeTTL),
			AttemptCount: 0,
			MaxAttempts:  s.maxAttempts,
			ResendCount:  0,
			MaxResends:   s.maxResends,
		})
		if err != nil {
			return err
		}

		challengeID = challenge.ID

		if err := createPendingAction(txCtx, challenge); err != nil {
			return err
		}

		template, err := s.emailTemplateForPurpose(purpose)
		if err != nil {
			return err
		}

		return s.enqueueChallengeEmail(txCtx, challenge.ID, email, template, now)
	})
	if err != nil {
		return uuid.Nil, err
	}

	return challengeID, nil
}

func (s *Service) ResendChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	now time.Time,
) error {
	challenge, err := s.store.GetChallengeByID(ctx, challengeID)
	if err != nil {
		return err
	}

	if err := s.validateChallengeForResend(challenge, now); err != nil {
		return err
	}

	ok, err := s.store.IncrementChallengeResendCount(ctx, challengeID, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTooManyResends
	}

	template, err := s.emailTemplateForPurpose(challenge.Purpose)
	if err != nil {
		return err
	}

	return s.enqueueChallengeEmail(ctx, challengeID, challenge.Email, template, now)
}

func (s *Service) verifyChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
) (*domain.Challenge, error) {
	challenge, err := s.store.GetChallengeByID(ctx, challengeID)
	if err != nil {
		return nil, err
	}

	if err := s.validateChallengeForVerify(challenge, now); err != nil {
		return nil, err
	}

	if err := verifier.VerifyCode(ctx, challengeID, code, now); err != nil {
		_ = s.store.IncrementChallengeAttemptCount(ctx, challengeID)
		return nil, err
	}

	if err := s.store.ConsumeChallenge(ctx, challengeID, now); err != nil {
		return nil, err
	}

	return &challenge, nil
}

func (s *Service) validateChallengeForVerify(
	challenge domain.Challenge,
	now time.Time,
) error {
	if challenge.IsConsumed() {
		return ErrChallengeConsumed
	}
	if challenge.IsExpired(now) {
		return ErrChallengeExpired
	}
	if !challenge.HasAttemptsRemaining() {
		return ErrTooManyAttempts
	}

	return nil
}

func (s *Service) validateChallengeForResend(
	challenge domain.Challenge,
	now time.Time,
) error {
	if challenge.IsConsumed() {
		return ErrChallengeConsumed
	}
	if challenge.IsExpired(now) {
		return ErrChallengeExpired
	}
	if challenge.ResendCount >= challenge.MaxResends {
		return ErrTooManyResends
	}
	if challenge.LastSentAt != nil && now.Before(challenge.LastSentAt.Add(s.minResendInterval)) {
		return ErrResendTooSoon
	}

	return nil
}

func (s *Service) enqueueChallengeEmail(
	ctx context.Context,
	challengeID uuid.UUID,
	toEmail string,
	template domain.EmailTemplate,
	now time.Time,
) error {
	_, err := s.store.CreateEmailJob(ctx, domain.EmailJob{
		ChallengeID:   &challengeID,
		ToEmail:       toEmail,
		Template:      template,
		Status:        domain.EmailJobStatusPending,
		AttemptCount:  0,
		NextAttemptAt: now,
	})
	return err
}

func (s *Service) emailTemplateForPurpose(
	purpose domain.ChallengePurpose,
) (domain.EmailTemplate, error) {
	switch purpose {
	case domain.ChallengePurposeSignup:
		return domain.EmailTemplateSignupCode, nil
	case domain.ChallengePurposePasswordReset:
		return domain.EmailTemplatePasswordResetCode, nil
	default:
		return "", ErrUnsupportedChallengePurpose
	}
}
