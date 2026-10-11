package viewmodel

import (
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/oauth"
)

func TestAuthProvidersFromDomainIncludesMicrosoft(t *testing.T) {
	configured := []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderGoogle, "google-client", "https://auth.example.com"),
		oauth.NewOAuthProvider(domain.ProviderApple, "apple-client", "https://auth.example.com"),
		oauth.NewOAuthProvider(domain.ProviderMicrosoft, "microsoft-client", "https://auth.example.com"),
	}

	unlinked := AuthProvidersFromDomain(nil, configured)
	if len(unlinked) != 4 || unlinked[3].Kind != AuthProviderMicrosoft || unlinked[3].Linked || unlinked[3].ActionLabel != "Link Microsoft" {
		t.Fatalf("unexpected unlinked providers: %+v", unlinked)
	}

	linked := AuthProvidersFromDomain([]domain.AuthProvider{{Provider: domain.ProviderMicrosoft}}, configured)
	if len(linked) != 4 || linked[3].Kind != AuthProviderMicrosoft || !linked[3].Linked || linked[3].ActionLabel != "Unlink" {
		t.Fatalf("unexpected linked providers: %+v", linked)
	}
}
