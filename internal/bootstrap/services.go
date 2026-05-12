package bootstrap

import (
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
)

type Services struct {
	Auth           *auth.Service
	Session        *session.Service
	Challenge      *challenge.Service
	Verification   *challenge.VerificationCodeService
	EmailWorker    *challenge.Worker
	OAuthProviders oauth.OAuthProviders
}

func NewServices(app *App) Services {
	txManager := tx.New(app.Store)
	accessPolicy := newAccessPolicy(app)
	oauthProviders := newOAuthProviders(app.Config)

	accessTokenService := token.NewAccessTokenService(
		app.Config.Token.KeySet,
		app.Config.Token.Issuer,
		app.Config.Token.AccessTokenTTL,
	)

	authService := auth.New(auth.Config{
		Store:            app.Store,
		Tx:               txManager,
		OAuthProviders:   oauthProviders,
		WebhookPublisher: newWebhookPublisher(app.Config),
		Logger:           app.Logger,
		AccessPolicy:     accessPolicy,
	})

	sessionService := session.New(session.SessionConfig{
		Store:                app.Store,
		Tx:                   txManager,
		AccessTokens:         accessTokenService,
		SessionTTL:           app.Config.Session.SessionTTL,
		RefreshTokenTTL:      app.Config.Session.RefreshTokenTTL,
		RefreshTokenRotation: app.Config.Session.RefreshTokenRotation,
		AccessPolicy:         accessPolicy,
	})

	verificationCodeService := newVerificationCodeService(app)
	challengeService := challenge.New(challenge.Config{
		Store:             app.Store,
		Tx:                txManager,
		ChallengeTTL:      app.Config.Challenge.TTL,
		MaxAttempts:       app.Config.Challenge.MaxAttempts,
		MaxResends:        app.Config.Challenge.MaxResends,
		MinResendInterval: app.Config.Challenge.MinResendInterval,
	})

	emailWorker := challenge.NewWorker(
		app.Store,
		verificationCodeService,
		newEmailSender(app.Config, app.Logger),
		app.Logger,
		challenge.WorkerConfig{
			WorkerCount:        app.Config.Email.WorkerCount,
			PollInterval:       app.Config.Email.WorkerPollInterval,
			JobMaxAttempts:     app.Config.Email.JobMaxAttempts,
			CleanupSentAfter:   app.Config.Email.CleanupSentAfter,
			CleanupFailedAfter: app.Config.Email.CleanupFailedAfter,
			CleanupInterval:    time.Hour,
			SendTimeout:        app.Config.Email.SMTPTimeout,
		},
	)

	return Services{
		Auth:           authService,
		Session:        sessionService,
		Challenge:      challengeService,
		Verification:   verificationCodeService,
		EmailWorker:    emailWorker,
		OAuthProviders: oauthProviders,
	}
}
