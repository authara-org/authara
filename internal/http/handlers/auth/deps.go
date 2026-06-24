package auth

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

type Deps struct {
	Admin          *admin.Service
	Auth           *auth.Service
	Passkeys       *passkey.Service
	Session        *session.Service
	Organizations  *organization.Service
	Challenge      *challenge.Service
	Features       features.Features
	Verification   *challenge.VerificationCodeService
	Limiter        ratelimiter.AuthLimiter
	Logger         *slog.Logger
	Google         *google.Client
	OAuthProviders oauth.OAuthProviders

	AccessTTL  time.Duration
	RefreshTTL time.Duration

	Render render.Renderer
}
