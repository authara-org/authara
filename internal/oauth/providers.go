package oauth

import (
	"strings"

	"github.com/authara-org/authara/internal/domain"
)

type CallbackURI string

type OAuthProvider struct {
	Name        domain.Provider
	ClientID    string
	RedirectURI string
}

func NewOAuthProvider(providerName domain.Provider, clientID, appURL string) OAuthProvider {
	return OAuthProvider{
		Name:        providerName,
		ClientID:    clientID,
		RedirectURI: strings.TrimRight(appURL, "/") + "/auth/oauth/" + string(providerName) + "/callback",
	}
}

type OAuthProviders struct {
	Providers []OAuthProvider
}
