package http

import (
	"context"
	"io"
	"log/slog"
	"time"

	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/features"
	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/handlers/ui"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func newTestHandlers(logger *slog.Logger, renderer render.Renderer) Handlers {
	return newTestHandlersWithAdmin(logger, renderer, nil, features.Features{})
}

func newTestHandlersWithAdmin(
	logger *slog.Logger,
	renderer render.Renderer,
	admin *adminsvc.Service,
	features features.Features,
) Handlers {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if renderer == nil {
		renderer = render.New(render.Assets{}, features.ChallengeEnabled)
	}

	googleClient := google.New("test-client-id")
	return Handlers{
		UI: ui.New(
			admin,
			nil,
			nil,
			nil,
			nil,
			nil,
			features,
			nil,
			email.NewTemplateService(testEmailTemplateStore{}),
			nil,
			logger,
			googleClient,
			oauth.OAuthProviders{},
			10*time.Minute,
			24*time.Hour,
			renderer,
		),
		API: api.New(
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			logger,
			googleClient,
			oauth.OAuthProviders{},
			features.ChallengeEnabled,
			features.UsernameLoginEnabled,
			10*time.Minute,
			24*time.Hour,
		),
		InternalAPI: internalapi.New(nil, organization.New(organization.Config{Mode: organization.OrgModeMulti}), false),
	}
}

type testEmailTemplateStore struct{}

func (testEmailTemplateStore) GetEmailTemplateOverride(context.Context, domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	return domain.EmailTemplateOverride{}, store.ErrEmailTemplateOverrideNotFound
}

func (testEmailTemplateStore) ListEmailTemplateOverrides(context.Context) ([]domain.EmailTemplateOverride, error) {
	return nil, nil
}

func (testEmailTemplateStore) ListEmailTemplateDeliverySettings(context.Context) ([]domain.EmailTemplateDeliverySetting, error) {
	return nil, nil
}

func (testEmailTemplateStore) SetEmailTemplateDeliveryEnabled(
	context.Context,
	domain.EmailTemplate,
	bool,
	uuid.UUID,
) (domain.EmailTemplateDeliverySetting, error) {
	return domain.EmailTemplateDeliverySetting{}, nil
}

func (testEmailTemplateStore) GetEmailTemplateVersion(context.Context, domain.EmailTemplate, int64) (domain.EmailTemplateVersion, error) {
	return domain.EmailTemplateVersion{}, store.ErrEmailTemplateVersionNotFound
}

func (testEmailTemplateStore) ListEmailTemplateVersions(context.Context, domain.EmailTemplate) ([]domain.EmailTemplateVersion, error) {
	return nil, nil
}

func (testEmailTemplateStore) UpsertEmailTemplateOverride(context.Context, domain.EmailTemplateOverride, int64) (domain.EmailTemplateOverride, error) {
	return domain.EmailTemplateOverride{}, nil
}

func (testEmailTemplateStore) DeleteEmailTemplateOverride(context.Context, domain.EmailTemplate, int64, uuid.UUID) error {
	return nil
}

func (testEmailTemplateStore) ListOperatorAuditEvents(context.Context, store.OperatorAuditEventFilter) ([]domain.OperatorAuditEvent, error) {
	return nil, nil
}
