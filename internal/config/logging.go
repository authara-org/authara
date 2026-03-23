package config

import (
	"fmt"
	"strings"
)

type Logging struct {
	Level string `env:"LOG_LEVEL"`
}

func (l *Logging) validate() error {
	lvl := strings.ToLower(l.Level)

	if lvl == "" {
		return nil
	}

	switch lvl {
	case "debug", "info", "warn", "error":
		l.Level = lvl
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q (allowed: debug, info, warn, error)", l.Level)
	}

	return nil
}

func (l *Logging) parse(appEnv string) error {
	if l.Level != "" {
		return nil
	}

	switch appEnv {
	case "dev":
		l.Level = "debug"
	case "prod":
		l.Level = "info"
	default:
		l.Level = "debug"
	}
	return nil
}
