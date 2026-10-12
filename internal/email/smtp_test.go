package email

import (
	"strings"
	"testing"
)

func TestBuildMIMEMessageIncludesMessageID(t *testing.T) {
	raw, err := buildMIMEMessage(
		"sender@example.test",
		"recipient@example.test",
		Message{MessageID: "<job-id@authara.local>", Subject: "test", Text: "test"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\r\nMessage-ID: <job-id@authara.local>\r\n") {
		t.Fatalf("message does not contain Message-ID header:\n%s", raw)
	}
}

func TestBuildMIMEMessageRejectsInvalidMessageID(t *testing.T) {
	_, err := buildMIMEMessage(
		"sender@example.test",
		"recipient@example.test",
		Message{MessageID: "job-id", Subject: "test", Text: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "invalid Message-ID") {
		t.Fatalf("buildMIMEMessage error = %v, want invalid Message-ID", err)
	}
}
