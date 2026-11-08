package main

import (
	"context"
	"os"
	"strings"

	"github.com/authara-org/authara/internal/lifecycle"
	"github.com/authara-org/authara/internal/logging"
	"github.com/authara-org/authara/internal/operations"
)

var Version = "dev"

func main() {
	// Local readiness probe for Docker HEALTHCHECK.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := lifecycle.RunHealthcheck(context.Background()); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && operations.IsCommand(os.Args[1]) {
		if err := operations.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
			emergencyLogger().Error(logging.EventInternalOperation,
				logging.ComponentField(logging.ComponentProcess),
				logging.OperationField(logging.OperationOther),
				logging.OutcomeField(logging.OutcomeFailed),
				logging.ErrorCodeField(logging.ErrorInternalFailure),
			)
			os.Exit(1)
		}
		return
	}

	if err := lifecycle.Run(Version); err != nil {
		emergencyLogger().Error(logging.EventProcessLifecycle,
			logging.ComponentField(logging.ComponentProcess),
			logging.OperationField(logging.OperationStartup),
			logging.OutcomeField(logging.OutcomeFailed),
			logging.ErrorCodeField(logging.ErrorConfigurationInvalid),
		)
		os.Exit(1)
	}
}

func emergencyLogger() *logging.Logger {
	environment := strings.TrimSpace(os.Getenv("AUTHARA_LOG_ENVIRONMENT"))
	if environment == "" {
		environment = strings.TrimSpace(os.Getenv("APP_ENV"))
	}
	if environment == "" {
		environment = "unknown"
	}
	logger, err := logging.New(logging.Config{Level: string(logging.LevelInfo), Environment: environment, Version: Version, Writer: os.Stderr})
	if err == nil {
		return logger
	}
	logger, _ = logging.New(logging.Config{Level: string(logging.LevelInfo), Environment: "unknown", Version: "unknown", Writer: os.Stderr})
	return logger
}
