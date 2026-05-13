package email

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNoopSenderDoesNotLogEmailBodies(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	sender := NewNoopSender(NoopSenderConfig{Logger: logger})

	err := sender.Send(context.Background(), "user@example.com", Message{
		Subject: "Verification",
		Text:    "Your code is 123456",
		HTML:    "<p>Your code is 123456</p>",
	})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	logged := buf.String()
	if strings.Contains(logged, "123456") || strings.Contains(logged, "Your code is") {
		t.Fatalf("noop sender logged sensitive email body: %s", logged)
	}
	if !strings.Contains(logged, "has_text=true") || !strings.Contains(logged, "has_html=true") {
		t.Fatalf("expected noop sender metadata in log, got: %s", logged)
	}
}
