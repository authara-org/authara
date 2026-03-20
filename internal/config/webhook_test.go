package config

import "testing"

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
	w := Webhook{
		URLRaw:        "https://example.com/webhooks/authara",
		Secret:        "secret",
		EnabledEvents: []string{"user.created", " user.deleted "},
		TimeoutRaw:    "5s",
	}

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
