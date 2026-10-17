package oauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func TestOAuthChallengeIsProviderBoundAndSingleUse(t *testing.T) {
	store := &memoryChallengeStore{}
	service := NewChallengeService(store, inlineTransaction{})
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	created, err := service.Create(context.Background(), domain.ProviderGoogle, now)
	if err != nil {
		t.Fatal(err)
	}
	if created.State == "" || created.Nonce == "" || created.State == created.Nonce {
		t.Fatalf("invalid challenge values: %#v", created)
	}
	if _, err := service.Consume(context.Background(), created.ID, domain.ProviderMicrosoft, "", now); !errors.Is(err, ErrInvalidAuthenticationChallenge) {
		t.Fatalf("wrong provider error = %v", err)
	}

	consumed, err := service.Consume(context.Background(), created.ID, domain.ProviderGoogle, "", now)
	if err != nil || consumed.Nonce != created.Nonce {
		t.Fatalf("consume = %#v, %v", consumed, err)
	}
	if _, err := service.Consume(context.Background(), created.ID, domain.ProviderGoogle, "", now); !errors.Is(err, ErrInvalidAuthenticationChallenge) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestOAuthChallengeRejectsWrongStateAndExpiry(t *testing.T) {
	store := &memoryChallengeStore{}
	service := NewChallengeService(store, inlineTransaction{})
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	created, err := service.Create(context.Background(), domain.ProviderApple, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Consume(context.Background(), created.ID, domain.ProviderApple, "wrong", now); !errors.Is(err, ErrInvalidAuthenticationChallenge) {
		t.Fatalf("wrong state error = %v", err)
	}
	if _, err := service.Consume(context.Background(), created.ID, domain.ProviderApple, created.State, created.ExpiresAt); !errors.Is(err, ErrInvalidAuthenticationChallenge) {
		t.Fatalf("expiry error = %v", err)
	}
}

type inlineTransaction struct{}

func (inlineTransaction) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type memoryChallengeStore struct {
	challenge domain.OAuthAuthenticationChallenge
}

func (s *memoryChallengeStore) CreateOAuthAuthenticationChallenge(_ context.Context, challenge domain.OAuthAuthenticationChallenge) (domain.OAuthAuthenticationChallenge, error) {
	challenge.ID = uuid.New()
	challenge.CreatedAt = challenge.ExpiresAt.Add(-oauthChallengeTTL)
	s.challenge = challenge
	return challenge, nil
}

func (s *memoryChallengeStore) GetOAuthAuthenticationChallengeForUpdate(_ context.Context, id uuid.UUID) (domain.OAuthAuthenticationChallenge, error) {
	if id != s.challenge.ID {
		return domain.OAuthAuthenticationChallenge{}, errors.New("not found")
	}
	return s.challenge, nil
}

func (s *memoryChallengeStore) ConsumeOAuthAuthenticationChallenge(_ context.Context, id uuid.UUID, now time.Time) error {
	if id != s.challenge.ID || s.challenge.ConsumedAt != nil {
		return errors.New("already consumed")
	}
	s.challenge.ConsumedAt = &now
	return nil
}
