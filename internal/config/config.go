package config

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Env string `env:"ENV,default=dev"`
	Dev bool   `env:"DEV_MODE,default=true"`

	DB      DB
	HTTP    HTTP
	Auth    Auth
	Logging Logging

	Google Google
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process(context.Background(), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
