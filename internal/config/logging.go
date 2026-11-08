package config

import (
	"fmt"
	"regexp"
	"strings"
)

type Logging struct {
	Level       string `env:"LOG_LEVEL"`
	Environment string `env:"AUTHARA_LOG_ENVIRONMENT"`
}

var logEnvironmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

func (l *Logging) validate() error {
	lvl := strings.ToLower(l.Level)

	switch lvl {
	case "":
	case "debug", "info", "warn", "error":
		l.Level = lvl
	default:
		return fmt.Errorf("invalid LOG_LEVEL %q (allowed: debug, info, warn, error)", l.Level)
	}

	if l.Environment != "" && !logEnvironmentPattern.MatchString(l.Environment) {
		return fmt.Errorf("invalid AUTHARA_LOG_ENVIRONMENT (expected 1-32 letters, numbers, dots, underscores, or hyphens)")
	}

	return nil
}

func (l *Logging) parse(appEnv string) error {
	if l.Environment == "" {
		l.Environment = appEnv
	}
	if l.Level == "" {
		switch appEnv {
		case "dev":
			l.Level = "debug"
		case "prod":
			l.Level = "info"
		default:
			l.Level = "debug"
		}
	}
	return nil
}
