package accesspolicy

import (
	"context"
	"strings"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
)

type EmailAccessPolicy interface {
	IsEmailAllowed(ctx context.Context, email string) (bool, error)
}

type Config struct {
	Store   *store.Store
	Enabled bool
	Policy  config.AllowlistPolicyReader
}

type Service struct {
	store  *store.Store
	policy config.AllowlistPolicyReader
}

func New(cfg Config) *Service {
	policy := cfg.Policy
	if policy == nil {
		policy = config.AllowlistPolicyReaderFunc(func() config.AllowlistPolicy {
			return config.AllowlistPolicy{AllowlistEnabled: cfg.Enabled}
		})
	}
	return &Service{
		store:  cfg.Store,
		policy: policy,
	}
}

func (s *Service) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	if !s.policy.CurrentAllowlist().AllowlistEnabled {
		return true, nil
	}

	email = normalize(email)

	return s.store.IsEmailAllowed(ctx, email)
}

func (s *Service) AllowEmail(ctx context.Context, email string) error {
	if !s.policy.CurrentAllowlist().AllowlistEnabled {
		return nil
	}

	email = normalize(email)
	if email == "" {
		return nil
	}

	return s.store.EnsureAllowedEmail(ctx, email)
}

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
