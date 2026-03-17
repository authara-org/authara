package oauth

type CallbackURI string

type Provider string

const (
	GoogleOAuth Provider = "google"
)

type OAuthProvider struct {
	Name     Provider
	ClientID string
}

func NewOAuthProvider(providerName Provider, clientID, appURL string) OAuthProvider {
	return OAuthProvider{
		Name:     providerName,
		ClientID: clientID,
	}
}

type OAuthProviders struct {
	Providers []OAuthProvider
}
