package challenge

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type Config struct {
	Store                  *store.Store
	Tx                     *tx.Manager
	AllowlistEnabled       bool
	ChallengeTTL           time.Duration
	MaxAttempts            int
	MaxResends             int
	MinResendInterval      time.Duration
	Policy                 config.ChallengePolicyReader
	WebhookPublisher       webhook.Publisher
	AccessTokenRevocations *token.AccessTokenRevocations
}

type Service struct {
	store                  *store.Store
	tx                     *tx.Manager
	allowlistEnabled       bool
	policy                 config.ChallengePolicyReader
	webhookPublisher       webhook.Publisher
	accessTokenRevocations *token.AccessTokenRevocations
}

func New(cfg Config) *Service {
	pub := cfg.WebhookPublisher
	if pub == nil {
		pub = webhook.NoopPublisher{}
	}

	policy := cfg.Policy
	if policy == nil {
		policy = config.StaticChallengePolicy{Policy: config.ChallengePolicy{
			Enabled: true, TTL: cfg.ChallengeTTL, VerificationCodeTTL: cfg.ChallengeTTL,
			MaxAttempts: cfg.MaxAttempts, MaxResends: cfg.MaxResends,
			MinimumResendInterval: cfg.MinResendInterval,
		}}
	}

	return &Service{
		store:                  cfg.Store,
		tx:                     cfg.Tx,
		allowlistEnabled:       cfg.AllowlistEnabled,
		policy:                 policy,
		webhookPublisher:       pub,
		accessTokenRevocations: cfg.AccessTokenRevocations,
	}
}

func (s *Service) CreateOpaqueChallenge(
	ctx context.Context,
	now time.Time,
	purpose domain.ChallengePurpose,
	email string,
) (uuid.UUID, error) {
	policy := s.policy.Current()
	var challengeID uuid.UUID

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, err := s.store.CreateChallenge(txCtx, domain.Challenge{
			Purpose:                  purpose,
			Email:                    email,
			ExpiresAt:                now.Add(policy.TTL),
			AttemptCount:             0,
			MaxAttempts:              policy.MaxAttempts,
			ResendCount:              0,
			MaxResends:               0,
			MinimumResendInterval:    policy.MinimumResendInterval,
			HasMinimumResendInterval: true,
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
	policy := s.policy.Current()
	var challengeID uuid.UUID

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, err := s.store.CreateChallenge(txCtx, domain.Challenge{
			Purpose:                  purpose,
			Email:                    email,
			ExpiresAt:                now.Add(policy.TTL),
			AttemptCount:             0,
			MaxAttempts:              policy.MaxAttempts,
			ResendCount:              0,
			MaxResends:               policy.MaxResends,
			MinimumResendInterval:    policy.MinimumResendInterval,
			HasMinimumResendInterval: true,
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
	policy := s.policy.Current()
	var resultErr error

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		challenge, err := s.store.GetChallengeByIDForUpdate(txCtx, challengeID)
		if err != nil {
			return err
		}

		if err := s.validateChallengeForResend(challenge, now, policy.MinimumResendInterval); err != nil {
			resultErr = err
			return nil
		}

		ok, err := s.store.IncrementChallengeResendCount(txCtx, challengeID, now)
		if err != nil {
			return err
		}
		if !ok {
			resultErr = ErrTooManyResends
			return nil
		}

		template, err := s.emailTemplateForPurpose(challenge.Purpose)
		if err != nil {
			return err
		}

		return s.enqueueChallengeEmail(txCtx, challengeID, challenge.Email, template, now)
	})
	if err != nil {
		return err
	}
	return resultErr
}

func (s *Service) verifyChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	purpose domain.ChallengePurpose,
	code string,
	verifier *VerificationCodeService,
	now time.Time,
	afterVerify func(context.Context, domain.Challenge) error,
) (*domain.Challenge, error) {
	var challenge domain.Challenge
	var resultErr error

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		row, err := s.store.GetChallengeByIDForUpdate(txCtx, challengeID)
		if err != nil {
			return err
		}
		challenge = row

		if err := s.validateChallengeForVerify(challenge, now); err != nil {
			resultErr = err
			return nil
		}
		if challenge.Purpose != purpose {
			resultErr = ErrUnsupportedChallengePurpose
			return nil
		}

		if err := verifier.VerifyCode(txCtx, challengeID, code, now); err != nil {
			if errors.Is(err, ErrInvalidVerificationCode) {
				if incErr := s.store.IncrementChallengeAttemptCount(txCtx, challengeID); incErr != nil {
					return incErr
				}
			}
			resultErr = err
			return nil
		}
		if afterVerify != nil {
			if err := afterVerify(txCtx, challenge); err != nil {
				return err
			}
		}

		if err := s.store.ConsumeChallenge(txCtx, challengeID, now); err != nil {
			if err == store.ErrorChallengeAlreadyConsumed {
				resultErr = ErrChallengeConsumed
				return nil
			}
			return err
		}

		challenge.ConsumedAt = &now
		return nil
	})
	if err != nil {
		return nil, err
	}
	if resultErr != nil {
		return nil, resultErr
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
	legacyMinimumResendInterval time.Duration,
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
	minimumResendInterval := challenge.MinimumResendInterval
	if !challenge.HasMinimumResendInterval {
		minimumResendInterval = legacyMinimumResendInterval
	}
	if challenge.LastSentAt != nil && now.Before(challenge.LastSentAt.Add(minimumResendInterval)) {
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
	case domain.ChallengePurposeEmailChange:
		return domain.EmailTemplateEmailChangeCode, nil
	default:
		return "", ErrUnsupportedChallengePurpose
	}
}
