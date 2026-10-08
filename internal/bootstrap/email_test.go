package bootstrap

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/config"
)

func TestWarnIfEmailDeliveryUnavailable(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))

	warnIfEmailDeliveryUnavailable(&config.Config{
		Email: config.Email{Provider: "noop"},
	}, logger)

	logged := output.String()
	if !strings.Contains(logged, "password recovery and security emails will not reach recipients") ||
		!strings.Contains(logged, "provider=noop") {
		t.Fatalf("warning = %q, want explicit recovery warning and provider", logged)
	}
}

func TestWarnIfEmailDeliveryUnavailableSkipsDeliverableProvider(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))

	warnIfEmailDeliveryUnavailable(&config.Config{
		Email: config.Email{Provider: "smtp"},
	}, logger)

	if logged := output.String(); logged != "" {
		t.Fatalf("unexpected warning for deliverable provider: %q", logged)
	}
}
