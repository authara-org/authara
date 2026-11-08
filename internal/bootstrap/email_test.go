package bootstrap

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/logging"
)

func TestWarnIfEmailDeliveryUnavailable(t *testing.T) {
	var output bytes.Buffer
	logger := newBootstrapTestLogger(t, &output)

	warnIfEmailDeliveryUnavailable(&config.Config{
		Email: config.Email{Provider: "noop"},
	}, logger)

	logged := output.String()
	for _, want := range []string{`"event":"configuration_status"`, `"component":"email"`, `"operation":"configuration"`, `"outcome":"degraded"`, `"error_code":"configuration_invalid"`} {
		if !strings.Contains(logged, want) {
			t.Fatalf("warning = %q, want structured field %s", logged, want)
		}
	}
}

func TestWarnIfEmailDeliveryUnavailableSkipsDeliverableProvider(t *testing.T) {
	var output bytes.Buffer
	logger := newBootstrapTestLogger(t, &output)

	warnIfEmailDeliveryUnavailable(&config.Config{
		Email: config.Email{Provider: "smtp"},
	}, logger)

	if logged := output.String(); logged != "" {
		t.Fatalf("unexpected warning for deliverable provider: %q", logged)
	}
}

func newBootstrapTestLogger(t *testing.T, writer io.Writer) *logging.Logger {
	t.Helper()
	logger, err := logging.New(logging.Config{Level: "debug", Environment: "test", Version: "test", Writer: writer})
	if err != nil {
		t.Fatal(err)
	}
	return logger
}
