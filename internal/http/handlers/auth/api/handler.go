package api

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type APIHandler struct {
	Auth    *auth.Service
	Session *session.Service
	Limiter ratelimiter.AuthLimiter
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
