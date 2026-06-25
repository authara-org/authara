package http

import (
	"io"
	"log/slog"
	"time"

	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/features"
	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/handlers/ui"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
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
			logger,
			googleClient,
			features.ChallengeEnabled,
			10*time.Minute,
			24*time.Hour,
		),
		InternalAPI: internalapi.New(nil),
	}
}
