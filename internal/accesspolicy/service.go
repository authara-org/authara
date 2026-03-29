package accesspolicy

import (
	"context"
	"strings"

	"github.com/authara-org/authara/internal/store"
)

type EmailAccessPolicy interface {
	IsEmailAllowed(ctx context.Context, email string) (bool, error)
}

type Config struct {
	Store   *store.Store
	Enabled bool
}

type Service struct {
	store   *store.Store
	enabled bool
}

func New(cfg Config) *Service {
	return &Service{
		store:   cfg.Store,
		enabled: cfg.Enabled,
	}
}

func (s *Service) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	if !s.enabled {
		return true, nil
	}

	email = normalize(email)

	return s.store.IsEmailAllowed(ctx, email)
}

func normalize(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
