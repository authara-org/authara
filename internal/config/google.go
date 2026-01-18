package config

type Google struct {
	ClientID string `env:"GOOGLE_CLIENT_ID" required:"true"`
}
