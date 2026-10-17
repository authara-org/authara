package oauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const oauthChallengeTTL = 10 * time.Minute

var ErrInvalidAuthenticationChallenge = errors.New("invalid oauth authentication challenge")

type ChallengeStore interface {
	CreateOAuthAuthenticationChallenge(context.Context, domain.OAuthAuthenticationChallenge) (domain.OAuthAuthenticationChallenge, error)
	GetOAuthAuthenticationChallengeForUpdate(context.Context, uuid.UUID) (domain.OAuthAuthenticationChallenge, error)
	ConsumeOAuthAuthenticationChallenge(context.Context, uuid.UUID, time.Time) error
}

type TransactionManager interface {
	WithTransaction(context.Context, func(context.Context) error) error
}

type ChallengeService struct {
	store ChallengeStore
	tx    TransactionManager
}

func NewChallengeService(challengeStore ChallengeStore, tx TransactionManager) *ChallengeService {
	return &ChallengeService{store: challengeStore, tx: tx}
}

func (s *ChallengeService) Create(ctx context.Context, provider domain.Provider, now time.Time) (domain.OAuthAuthenticationChallenge, error) {
	state, err := randomChallengeValue()
	if err != nil {
		return domain.OAuthAuthenticationChallenge{}, err
	}
	nonce, err := randomChallengeValue()
	if err != nil {
		return domain.OAuthAuthenticationChallenge{}, err
	}
	return s.store.CreateOAuthAuthenticationChallenge(ctx, domain.OAuthAuthenticationChallenge{
		Provider:  provider,
		State:     state,
		Nonce:     nonce,
		ExpiresAt: now.UTC().Add(oauthChallengeTTL),
	})
}

func (s *ChallengeService) Consume(ctx context.Context, id uuid.UUID, provider domain.Provider, state string, now time.Time) (domain.OAuthAuthenticationChallenge, error) {
	var challenge domain.OAuthAuthenticationChallenge
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		challenge, err = s.store.GetOAuthAuthenticationChallengeForUpdate(txCtx, id)
		if err != nil {
			if errors.Is(err, store.ErrOAuthAuthenticationChallengeNotFound) {
				return ErrInvalidAuthenticationChallenge
			}
			return err
		}
		stateMismatch := state != "" && subtle.ConstantTimeCompare([]byte(challenge.State), []byte(state)) != 1
		if challenge.ConsumedAt != nil || !now.Before(challenge.ExpiresAt) || challenge.Provider != provider || stateMismatch {
			return ErrInvalidAuthenticationChallenge
		}
		if err := s.store.ConsumeOAuthAuthenticationChallenge(txCtx, id, now.UTC()); err != nil {
			if errors.Is(err, store.ErrOAuthAuthenticationChallengeConsumed) {
				return ErrInvalidAuthenticationChallenge
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domain.OAuthAuthenticationChallenge{}, err
	}
	return challenge, nil
}

func randomChallengeValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
