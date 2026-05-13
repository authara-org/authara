package bootstrap

import (
	"fmt"
	"net/url"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
)

type Services struct {
	Auth           *auth.Service
	Passkeys       *passkey.Service
	Session        *session.Service
	Challenge      *challenge.Service
	Verification   *challenge.VerificationCodeService
	EmailWorker    *challenge.Worker
	OAuthProviders oauth.OAuthProviders
}

func NewServices(app *App) (Services, error) {
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

	passkeyService, err := newPasskeyService(app, txManager)
	if err != nil {
		return Services{}, fmt.Errorf("create passkey service: %w", err)
	}

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
		Passkeys:       passkeyService,
		Session:        sessionService,
		Challenge:      challengeService,
		Verification:   verificationCodeService,
		EmailWorker:    emailWorker,
		OAuthProviders: oauthProviders,
	}, nil
}

func newPasskeyService(app *App, txManager *tx.Manager) (*passkey.Service, error) {
	publicURL, err := url.Parse(app.Config.Values.PublicURL)
	if err != nil {
		return nil, err
	}

	return passkey.New(passkey.Config{
		RPDisplayName: "Authara",
		RPID:          publicURL.Hostname(),
		RPOrigins:     []string{app.Config.Values.PublicURL},
		Store:         app.Store,
		Tx:            txManager,
		Logger:        app.Logger,
	})
}
