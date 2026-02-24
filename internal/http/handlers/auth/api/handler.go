package api

import (
	"log/slog"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	authhandler "github.com/alexlup06-authgate/authgate/internal/http/handlers/auth"
	"github.com/alexlup06-authgate/authgate/internal/oauth/google"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

type APIHandler struct {
	Auth    *auth.Service
	Session *session.Service
	Limiter ratelimit.AuthLimiter
	Logger  *slog.Logger
	Google  *google.Client

	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func NewAPIHandler(d authhandler.Deps) *APIHandler {
	return &APIHandler{
		Auth:    d.Auth,
		Session: d.Session,
		Limiter: d.Limiter,
		Logger:  d.Logger,
		Google:  d.Google,

		AccessTTL:  d.AccessTTL,
		RefreshTTL: d.RefreshTTL,
	}
}
