package auth

import (
	"log/slog"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/oauth/google"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

type Deps struct {
	Auth    *auth.Service
	Session *session.Service
	Limiter ratelimit.AuthLimiter
	Logger  *slog.Logger
	Google  *google.Client

	AccessTTL  time.Duration
	RefreshTTL time.Duration
}
