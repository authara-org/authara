package auth

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
)

type Deps struct {
	Auth             *auth.Service
	Session          *session.Service
	Challenge        *challenge.Service
	ChallengeEnabled bool
	Verification     *challenge.VerificationCodeService
	Limiter          ratelimiter.AuthLimiter
	Logger           *slog.Logger
	Google           *google.Client
	OAuthProviders   oauth.OAuthProviders

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	Render render.Renderer
}
