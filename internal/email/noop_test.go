package email

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/logging"
)

func TestNoopSenderLogsOnlyNonSensitiveMetadata(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logging.New(logging.Config{Level: "debug", Environment: "test", Version: "test", Writer: &buf})
	if err != nil {
		t.Fatal(err)
	}
	sender := NewNoopSender(NoopSenderConfig{Logger: logger})

	recipient := "private-recipient@example.com"
	subject := "Verification for private-subject@example.com"
	err = sender.Send(context.Background(), recipient, Message{
		Subject: subject,
		Text:    "Your code is 123456",
		HTML:    "<p>Your code is 123456</p>",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	logged := buf.String()
	for _, sensitive := range []string{recipient, subject, "123456", "Your code is"} {
		if strings.Contains(logged, sensitive) {
			t.Fatalf("noop sender logged sensitive value %q: %s", sensitive, logged)
		}
	}
	for _, want := range []string{
		`"event":"worker_outcome"`,
		`"worker":"email"`,
		`"operation":"deliver"`,
		`"outcome":"skipped"`,
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("expected noop sender event field %s in log, got: %s", want, logged)
		}
	}
}
