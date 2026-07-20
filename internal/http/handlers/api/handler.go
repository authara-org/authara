package api

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type APIHandler struct {
	Auth           *auth.Service
	Passkeys       *passkey.Service
	Session        *session.Service
	Organizations  *organization.Service
	Challenge      *challenge.Service
	Verification   *challenge.VerificationCodeService
	Limiter        ratelimiter.AuthLimiter
	Logger         *slog.Logger
	Google         *google.Client
	OAuthProviders oauth.OAuthProviders

	ChallengeEnabled bool
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
}

func New(
	auth *auth.Service,
	passkeys *passkey.Service,
	session *session.Service,
	organizations *organization.Service,
	challenge *challenge.Service,
	verification *challenge.VerificationCodeService,
	limiter ratelimiter.AuthLimiter,
	logger *slog.Logger,
	google *google.Client,
	oauthProviders oauth.OAuthProviders,
	challengeEnabled bool,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *APIHandler {
	return &APIHandler{
		Auth:             auth,
		Passkeys:         passkeys,
		Session:          session,
		Organizations:    organizations,
		Challenge:        challenge,
		Verification:     verification,
		Limiter:          limiter,
		Logger:           logger,
		Google:           google,
		OAuthProviders:   oauthProviders,
		ChallengeEnabled: challengeEnabled,
		AccessTTL:        accessTTL,
		RefreshTTL:       refreshTTL,
	}
}
