package bootstrap

import (
	"fmt"
	"net/url"
	"time"

	"github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
)

type Services struct {
	Admin          *admin.Service
	Auth           *auth.Service
	Passkeys       *passkey.Service
	Session        *session.Service
	Organizations  *organization.Service
	Challenge      *challenge.Service
	Verification   *challenge.VerificationCodeService
	EmailTemplates *email.TemplateService
	EmailWorker    *challenge.Worker
	WebhookWorker  *webhook.Worker
	OAuthProviders oauth.OAuthProviders
}

func NewServices(app *App) (Services, error) {
	if app == nil {
		return Services{}, fmt.Errorf("app is required")
	}
	if app.Config == nil {
		return Services{}, fmt.Errorf("config service is required")
	}
	txManager := tx.New(app.Store)
	accessPolicy := newAccessPolicy(app)
	oauthProviders := newOAuthProviders(app.Config.Startup())
	webhookPublisher := newWebhookPublisher(app.Config, app.Store)
	webhookWorker := newWebhookWorker(app.Config, app.Store, app.Logger, app.Observability)

	accessTokenService := token.NewAccessTokenServiceWithTTL(
		app.Config.Token.KeySet,
		app.Config.Token.Issuer,
		func() time.Duration { return app.Config.CurrentToken().AccessTokenTTL },
	)
	accessTokenRevocations := token.NewAccessTokenRevocationsWithTTL(
		app.Cache,
		func() time.Duration {
			// Operator-managed access-token lifetimes are capped at 24 hours.
			// Keep scope revocations for at least that long so lowering the
			// policy cannot let an older, longer-lived token outlast its marker.
			return max(app.Config.CurrentToken().AccessTokenTTL, 24*time.Hour)
		},
	)

	organizationService := organization.New(organization.Config{
		Store:                  app.Store,
		Tx:                     txManager,
		WebhookPublisher:       webhookPublisher,
		Logger:                 app.Logger,
		Policy:                 app.Config,
		PublicURL:              app.Config.Values.PublicURL,
		Mode:                   organization.OrgMode(app.Config.Organization.Mode),
		AccessTokenRevocations: accessTokenRevocations,
	})
	app.Logger.Warn("AUTHARA_ORG_MODE is a boot-time product shape; changing it after production use is unsupported", "mode", app.Config.Organization.Mode)

	authService := auth.New(auth.Config{
		Store:                  app.Store,
		Tx:                     txManager,
		OAuthProviders:         oauthProviders,
		WebhookPublisher:       webhookPublisher,
		Logger:                 app.Logger,
		AccessPolicy:           accessPolicy,
		Organizations:          organizationService,
		AccessTokenRevocations: accessTokenRevocations,
	})

	sessionService := session.New(session.SessionConfig{
		Store:                  app.Store,
		Tx:                     txManager,
		AccessTokens:           accessTokenService,
		AccessTokenRevocations: accessTokenRevocations,
		Policy:                 app.Config,
		AccessPolicy:           accessPolicy,
		Organizations:          organizationService,
	})

	adminService := admin.New(admin.Config{
		Store:                  app.Store,
		Tx:                     txManager,
		Policy:                 app.Config,
		AllowlistPolicy:        app.Config,
		WebhookPublisher:       webhookPublisher,
		AccessTokenRevocations: accessTokenRevocations,
	})

	passkeyService, err := newPasskeyService(app, txManager)
	if err != nil {
		return Services{}, fmt.Errorf("create passkey service: %w", err)
	}

	verificationCodeService := newVerificationCodeService(app)
	emailTemplateService := email.NewTemplateService(app.Store)
	challengeService := challenge.New(challenge.Config{
		Store:                  app.Store,
		Tx:                     txManager,
		Policy:                 app.Config,
		AllowlistPolicy:        app.Config,
		WebhookPublisher:       webhookPublisher,
		AccessTokenRevocations: accessTokenRevocations,
	})

	emailWorker := challenge.NewWorker(
		app.Store,
		verificationCodeService,
		emailTemplateService,
		newEmailSender(app.Config.Startup(), app.Logger),
		app.Logger,
		challenge.WorkerConfig{
			WorkerCount:     app.Config.Email.WorkerCount,
			PollInterval:    app.Config.Email.WorkerPollInterval,
			CleanupInterval: time.Hour,
			SendTimeout:     app.Config.Email.SMTPTimeout,
			Policy:          app.Config,
			Metrics:         app.Observability,
		},
	)

	return Services{
		Admin:          adminService,
		Auth:           authService,
		Passkeys:       passkeyService,
		Session:        sessionService,
		Organizations:  organizationService,
		Challenge:      challengeService,
		Verification:   verificationCodeService,
		EmailTemplates: emailTemplateService,
		EmailWorker:    emailWorker,
		WebhookWorker:  webhookWorker,
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
