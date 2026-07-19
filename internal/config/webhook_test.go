package config

import (
	"testing"
	"time"
)

func TestWebhook_EventEnabled(t *testing.T) {
	w := Webhook{
		URL:    "https://example.com/webhooks/authara",
		Secret: "secret",
		EnabledEventSet: map[string]struct{}{
			"user.created": {},
			"user.deleted": {},
		},
	}

	if !w.EventEnabled("user.created") {
		t.Fatal("expected user.created to be enabled")
	}
	if !w.EventEnabled("user.deleted") {
		t.Fatal("expected user.deleted to be enabled")
	}
	if w.EventEnabled("user.disabled") {
		t.Fatal("expected user.disabled to be disabled")
	}
}

func TestWebhook_EventEnabled_DisabledWhenWebhookNotConfigured(t *testing.T) {
	w := Webhook{}

	if w.EventEnabled("user.created") {
		t.Fatal("expected event to be disabled when webhook is not configured")
	}
}

func TestWebhook_EventEnabled_DefaultsToAllWhenConfiguredAndNoExplicitEvents(t *testing.T) {
	w := Webhook{
		URL:    "https://example.com/webhooks/authara",
		Secret: "secret",
	}

	if !w.EventEnabled("user.created") {
		t.Fatal("expected user.created to be enabled by default")
	}
	if !w.EventEnabled("user.deleted") {
		t.Fatal("expected user.deleted to be enabled by default")
	}
}

func TestWebhook_ParseBuildsEnabledEventSet(t *testing.T) {
	w := validWebhookConfig()
	w.URLRaw = "https://example.com/webhooks/authara"
	w.Secret = "secret"
	w.EnabledEvents = []string{"user.created", " user.deleted "}

	if err := w.validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if err := w.parse(); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !w.EventEnabled("user.created") {
		t.Fatal("expected user.created to be enabled")
	}
	if !w.EventEnabled("user.deleted") {
		t.Fatal("expected user.deleted to be enabled")
	}
	if w.EventEnabled("user.disabled") {
		t.Fatal("expected user.disabled to be disabled")
	}
}

func TestWebhook_ValidateRejectsInvalidWorkerCount(t *testing.T) {
	w := validWebhookConfig()
	w.WorkerCount = 0

	if err := w.validate(); err == nil {
		t.Fatal("expected zero webhook workers to be rejected")
	}
}

func TestWebhook_ValidateRejectsTimeoutAtStaleThreshold(t *testing.T) {
	w := validWebhookConfig()
	w.Timeout = w.ProcessingStaleAfter

	if err := w.validate(); err == nil {
		t.Fatal("expected webhook timeout at the stale threshold to be rejected")
	}
}

func validWebhookConfig() Webhook {
	return Webhook{
		Timeout:              5 * time.Second,
		WorkerCount:          2,
		MaxDeliveryAttempts:  3,
		ProcessingStaleAfter: 2 * time.Minute,
		StaleReaperInterval:  time.Minute,
		DeliveredRetention:   24 * time.Hour,
		FailedRetention:      30 * 24 * time.Hour,
		CleanupInterval:      time.Hour,
		MaintenanceBatchSize: 1000,
	}
}
