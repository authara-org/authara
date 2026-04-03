package ui

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type UIHandler struct {
	Auth         *auth.Service
	Session      *session.Service
	Challange    *challenge.Service
	Verification *challenge.VerificationCodeService

	Limiter        ratelimiter.AuthLimiter
	Logger         *slog.Logger
	Google         *google.Client
	OAuthProviders oauth.OAuthProviders

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	Render render.Renderer
}

func NewUIHandler(d authhandler.Deps) *UIHandler {
	return &UIHandler{
		Auth:           d.Auth,
		Session:        d.Session,
		Challange:      d.Challange,
		Verification:   d.Verification,
		Limiter:        d.Limiter,
		Logger:         d.Logger,
		Google:         d.Google,
		OAuthProviders: d.OAuthProviders,

		AccessTTL:  d.AccessTTL,
		RefreshTTL: d.RefreshTTL,

		Render: d.Render,
	}
}
