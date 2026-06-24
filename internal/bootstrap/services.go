package bootstrap

import (
	"fmt"
	"net/url"
	"time"

	"github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store/tx"
)

type Services struct {
	Admin          *admin.Service
	Auth           *auth.Service
	Passkeys       *passkey.Service
	Session        *session.Service
	Organizations  *organization.Service
	Challenge      *challenge.Service
	Verification   *challenge.VerificationCodeService
	EmailWorker    *challenge.Worker
	OAuthProviders oauth.OAuthProviders
}

func NewServices(app *App) (Services, error) {
	txManager := tx.New(app.Store)
	accessPolicy := newAccessPolicy(app)
	oauthProviders := newOAuthProviders(app.Config)
	webhookPublisher := newWebhookPublisher(app.Config)

	accessTokenService := token.NewAccessTokenService(
		app.Config.Token.KeySet,
		app.Config.Token.Issuer,
		app.Config.Token.AccessTokenTTL,
	)

	organizationService := organization.New(organization.Config{
		Store:            app.Store,
		Tx:               txManager,
		WebhookPublisher: webhookPublisher,
		Logger:           app.Logger,
		InvitationTTL:    app.Config.Organization.InvitationTTL,
		PublicURL:        app.Config.Values.PublicURL,
		Mode:             organization.OrgMode(app.Config.Organization.Mode),
	})
	app.Logger.Warn("AUTHARA_ORG_MODE is a boot-time product shape; changing it after production use is unsupported", "mode", app.Config.Organization.Mode)

	authService := auth.New(auth.Config{
		Store:            app.Store,
		Tx:               txManager,
		OAuthProviders:   oauthProviders,
		WebhookPublisher: webhookPublisher,
		Logger:           app.Logger,
		AccessPolicy:     accessPolicy,
		Organizations:    organizationService,
	})

	sessionService := session.New(session.SessionConfig{
		Store:                app.Store,
		Tx:                   txManager,
		AccessTokens:         accessTokenService,
		SessionTTL:           app.Config.Session.SessionTTL,
		RefreshTokenTTL:      app.Config.Session.RefreshTokenTTL,
		RefreshTokenRotation: app.Config.Session.RefreshTokenRotation,
		AccessPolicy:         accessPolicy,
		Organizations:        organizationService,
	})

	adminService := admin.New(admin.Config{
		Store:            app.Store,
		Tx:               txManager,
		AllowlistEnabled: app.Config.AccessPolicy.AllowedEmailEnabled,
		AuditRetention:   time.Duration(app.Config.Admin.AuditRetentionDays) * 24 * time.Hour,
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
		Admin:          adminService,
		Auth:           authService,
		Passkeys:       passkeyService,
		Session:        sessionService,
		Organizations:  organizationService,
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
