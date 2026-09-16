package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNoopSenderLogsOnlyNonSensitiveMetadata(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	sender := NewNoopSender(NoopSenderConfig{Logger: logger})

	recipient := "private-recipient@example.com"
	subject := "Verification for private-subject@example.com"
	err := sender.Send(context.Background(), recipient, Message{
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
	if !strings.Contains(logged, "has_subject=true") ||
		!strings.Contains(logged, "has_text=true") ||
		!strings.Contains(logged, "has_html=true") {
		t.Fatalf("expected noop sender metadata in log, got: %s", logged)
	}
}
