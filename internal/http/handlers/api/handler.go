package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/apple"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

type GoogleVerifier interface {
	VerifyIDToken(context.Context, string, string) (*google.Identity, error)
}

type AppleClient interface {
	Exchange(context.Context, string, string) (apple.ExchangeResult, error)
	Revoke(context.Context, string) error
}

type AppleCredentialStore interface {
	QueueRevocation(context.Context, string) error
	StageProviderLink(context.Context, uuid.UUID, string, time.Time) error
}

func (h *APIHandler) passwordPolicyError(err error) (response.ErrorCode, string) {
	message, ok := auth.PasswordPolicyMessage(err, h.Auth.PasswordMinimumLength())
	if !ok {
		return responseCodeInternalError(), "Password error."
	}
	return responseCodeInvalidRequest(), message
}

func (h *APIHandler) rateLimitResult(allowed bool, err error, limitedMessage string) (response.ErrorCode, string, bool) {
	if _, limited := ratelimiter.IsRateLimited(err); limited || (err == nil && !allowed) {
		return response.CodeRateLimited, limitedMessage, false
	}
	if err != nil {
		logger := h.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("rate limiter unavailable", "err", err)
		return response.CodeInternalError, "Authentication service unavailable.", false
	}
	return "", "", true
}

type APIHandler struct {
	Auth             *auth.Service
	Passkeys         *passkey.Service
	Session          *session.Service
	Organizations    *organization.Service
	Challenge        *challenge.Service
	Verification     *challenge.VerificationCodeService
	Limiter          ratelimiter.AuthLimiter
	Logger           *slog.Logger
	Google           GoogleVerifier
	Apple            AppleClient
	AppleCredentials AppleCredentialStore
	OAuthProviders   oauth.OAuthProviders
	Config           RuntimePolicyReader

	ChallengeEnabled     bool
	UsernameLoginEnabled bool
	AccessTTL            time.Duration
	RefreshTTL           time.Duration
}

type RuntimePolicyReader interface {
	config.AuthenticationPolicyReader
	config.SessionCookiePolicyReader
}

func (h *APIHandler) sessionCookiePolicy() config.SessionCookiePolicy {
	if h.Config != nil {
		return h.Config.CurrentSessionCookies()
	}
	return config.SessionCookiePolicy{AccessTokenTTL: h.AccessTTL, RefreshTokenTTL: h.RefreshTTL}
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
	google GoogleVerifier,
	oauthProviders oauth.OAuthProviders,
	configuration RuntimePolicyReader,
	challengeEnabled bool,
	usernameLoginEnabled bool,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *APIHandler {
	return &APIHandler{
		Auth:                 auth,
		Passkeys:             passkeys,
		Session:              session,
		Organizations:        organizations,
		Challenge:            challenge,
		Verification:         verification,
		Limiter:              limiter,
		Logger:               logger,
		Google:               google,
		OAuthProviders:       oauthProviders,
		Config:               configuration,
		ChallengeEnabled:     challengeEnabled,
		UsernameLoginEnabled: usernameLoginEnabled,
		AccessTTL:            accessTTL,
		RefreshTTL:           refreshTTL,
	}
}

func (h *APIHandler) usernameLoginEnabled() bool {
	if h.Config != nil {
		return h.Config.CurrentAuthentication().UsernameLoginEnabled
	}
	return h.UsernameLoginEnabled
}
