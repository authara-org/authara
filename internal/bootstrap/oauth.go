package bootstrap

import (
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/oauth"
)

func newOAuthProviders(cfg *config.Config) oauth.OAuthProviders {
	providers := []oauth.OAuthProvider{}
	for _, p := range cfg.OAuth.Providers {
		switch p {
		case string(domain.ProviderGoogle):
			providers = append(
				providers,
				oauth.NewOAuthProvider(domain.ProviderGoogle, cfg.OAuth.GoogleClientID, cfg.Values.PublicURL),
			)
		}
	}

	return oauth.OAuthProviders{Providers: providers}
}
