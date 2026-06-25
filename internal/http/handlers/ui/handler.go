package ui

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/features"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type UIHandler struct {
	Admin         *admin.Service
	Auth          *auth.Service
	Passkeys      *passkey.Service
	Session       *session.Service
	Organizations *organization.Service
	Challenge     *challenge.Service
	Features      features.Features
	Verification  *challenge.VerificationCodeService

	Limiter        ratelimiter.AuthLimiter
	Logger         *slog.Logger
	Google         *google.Client
	OAuthProviders oauth.OAuthProviders

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	Render render.Renderer
}

func New(
	admin *admin.Service,
	auth *auth.Service,
	passkeys *passkey.Service,
	session *session.Service,
	organizations *organization.Service,
	challenge *challenge.Service,
	features features.Features,
	verification *challenge.VerificationCodeService,
	limiter ratelimiter.AuthLimiter,
	logger *slog.Logger,
	google *google.Client,
	oauthProviders oauth.OAuthProviders,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	renderer render.Renderer,
) *UIHandler {
	return &UIHandler{
		Admin:          admin,
		Auth:           auth,
		Passkeys:       passkeys,
		Session:        session,
		Organizations:  organizations,
		Challenge:      challenge,
		Features:       features,
		Verification:   verification,
		Limiter:        limiter,
		Logger:         logger,
		Google:         google,
		OAuthProviders: oauthProviders,
		AccessTTL:      accessTTL,
		RefreshTTL:     refreshTTL,
		Render:         renderer,
	}
}
