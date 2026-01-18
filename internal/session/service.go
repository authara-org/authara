package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alexlup06/authgate/internal/domain"
	"github.com/alexlup06/authgate/internal/store"
)

var (
	ErrNoSession      = errors.New("no session")
	ErrSessionExpired = errors.New("session expired")
	ErrInvalidSession = errors.New("invalid session")
)

type Config struct {
	Store *store.Store

	// cookie / session settings (can be extended later)
	CookieName string
	TTL        time.Duration
	Secure     bool
}

type Service struct {
	store      *store.Store
	cookieName string
	ttl        time.Duration
	secure     bool
}

func New(cfg Config) *Service {
	cookieName := cfg.CookieName
	if cookieName == "" {
		cookieName = "authgate_session"
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	return &Service{
		store:      cfg.Store,
		cookieName: cookieName,
		ttl:        ttl,
		secure:     cfg.Secure,
	}
}

func (s *Service) Create(ctx context.Context, user domain.User) (*domain.Session, error) {
	return &domain.Session{}, nil
}

func (s *Service) Validate(ctx context.Context, r *http.Request) (*domain.User, error) {
	return &domain.User{}, nil
}

func (s *Service) Destroy(ctx context.Context, r *http.Request) error {
	return nil
}
