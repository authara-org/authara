package viewmodel

import (
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/oauth"
)

type AuthProviderKind string

const (
	AuthProviderPassword AuthProviderKind = "password"
	AuthProviderGoogle   AuthProviderKind = "google"
)

type AuthProviderFlow string

const (
	AuthProviderFlowLogin AuthProviderFlow = "login"
	AuthProviderFlowLink  AuthProviderFlow = "link"
	AuthProviderFlowProof AuthProviderFlow = "proof"
)

type AuthProvider struct {
	ID          string
	Kind        AuthProviderKind
	Title       string
	Subtitle    string
	Linked      bool
	Primary     bool
	ActionLabel string
	ActionURL   string
}

func AuthProvidersFromDomain(
	providers []domain.AuthProvider,
	oauthProviders []oauth.OAuthProvider,
) []AuthProvider {
	var (
		hasPassword   bool
		hasGoogle     bool
		googleEnabled bool
	)

	for _, p := range providers {
		switch p.Provider {
		case domain.ProviderPassword:
			hasPassword = true
		case domain.ProviderGoogle:
			hasGoogle = true
		}
	}

	for _, p := range oauthProviders {
		if p.Name == domain.ProviderGoogle {
			googleEnabled = true
			break
		}
	}

	out := make([]AuthProvider, 0, 2)

	if hasPassword {
		out = append(out, AuthProvider{
			ID:          "password",
			Kind:        AuthProviderPassword,
			Title:       "Password",
			Subtitle:    "Use your email and password to sign in.",
			Linked:      true,
			Primary:     true,
			ActionLabel: "Change password",
			ActionURL:   "/auth/providers/password/unlink",
		})
	} else {
		out = append(out, AuthProvider{
			ID:          "password",
			Kind:        AuthProviderPassword,
			Title:       "Password",
			Subtitle:    "Add a password to sign in with your email.",
			Linked:      false,
			Primary:     false,
			ActionLabel: "Add password",
			ActionURL:   "/auth/providers/password/link",
		})
	}

	if googleEnabled {
		if hasGoogle {
			out = append(out, AuthProvider{
				ID:          "google",
				Kind:        AuthProviderGoogle,
				Title:       "Google",
				Subtitle:    "Sign in with your Google account.",
				Linked:      true,
				Primary:     false,
				ActionLabel: "Unlink",
				ActionURL:   "/auth/providers/google/unlink",
			})
		} else {
			out = append(out, AuthProvider{
				ID:          "google",
				Kind:        AuthProviderGoogle,
				Title:       "Google",
				Subtitle:    "Connect your Google account for faster sign-in.",
				Linked:      false,
				Primary:     false,
				ActionLabel: "Link Google",
				ActionURL:   "/auth/providers/google/link",
			})
		}
	}

	return out
}
