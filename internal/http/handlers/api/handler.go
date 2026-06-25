package api

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type APIHandler struct {
	Auth          *auth.Service
	Session       *session.Service
	Organizations *organization.Service
	Limiter       ratelimiter.AuthLimiter
	Logger        *slog.Logger
	Google        *google.Client

	ChallengeEnabled bool
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
}

func New(
	auth *auth.Service,
	session *session.Service,
	organizations *organization.Service,
	limiter ratelimiter.AuthLimiter,
	logger *slog.Logger,
	google *google.Client,
	challengeEnabled bool,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *APIHandler {
	return &APIHandler{
		Auth:             auth,
		Session:          session,
		Organizations:    organizations,
		Limiter:          limiter,
		Logger:           logger,
		Google:           google,
		ChallengeEnabled: challengeEnabled,
		AccessTTL:        accessTTL,
		RefreshTTL:       refreshTTL,
	}
}
