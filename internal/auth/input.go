package auth

type Provider string

const (
	ProviderPassword Provider = "password"
	ProviderGoogle   Provider = "google"
	ProviderApple    Provider = "apple"
)

type SignupInput struct {
	Provider Provider

	Email    string
	Password string

	OAuthID string
}

type LoginInput struct {
	Provider Provider

	Email    string
	Password string

	OAuthID string
}
