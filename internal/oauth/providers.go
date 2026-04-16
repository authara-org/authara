package oauth

import "github.com/authara-org/authara/internal/domain"

type CallbackURI string

type OAuthProvider struct {
	Name     domain.Provider
	ClientID string
}

func NewOAuthProvider(providerName domain.Provider, clientID, appURL string) OAuthProvider {
	return OAuthProvider{
		Name:     providerName,
		ClientID: clientID,
	}
}

type OAuthProviders struct {
	Providers []OAuthProvider
}
