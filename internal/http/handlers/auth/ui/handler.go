package ui

import (
	"log/slog"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	authhandler "github.com/alexlup06-authgate/authgate/internal/http/handlers/auth"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/render"
	"github.com/alexlup06-authgate/authgate/internal/oauth/google"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

type UIHandler struct {
	Auth    *auth.Service
	Session *session.Service
	Limiter ratelimit.AuthLimiter
	Logger  *slog.Logger
	Google  *google.Client

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	Render render.Renderer
}

func NewUIHandler(d authhandler.Deps) *UIHandler {
	return &UIHandler{
		Auth:    d.Auth,
		Session: d.Session,
		Limiter: d.Limiter,
		Logger:  d.Logger,
		Google:  d.Google,

		AccessTTL:  d.AccessTTL,
		RefreshTTL: d.RefreshTTL,

		Render: d.Render,
	}
}
