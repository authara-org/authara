package config

import (
	"strings"
	"testing"
	"time"
)

func validEmailConfig() Email {
	return Email{
		Provider:             "smtp",
		From:                 "sender@example.test",
		SMTPHost:             "smtp.example.test",
		SMTPPort:             587,
		SMTPTLS:              true,
		SMTPTimeout:          10 * time.Second,
		WorkerCount:          1,
		WorkerPollInterval:   time.Second,
		JobMaxAttempts:       100,
		ProcessingStaleAfter: 2 * time.Minute,
		StaleReaperInterval:  time.Minute,
		MaintenanceBatchSize: 1000,
		CleanupSentAfter:     30 * 24 * time.Hour,
		CleanupFailedAfter:   90 * 24 * time.Hour,
	}
}

func TestEmailConfigRequiresCompleteCredentials(t *testing.T) {
	cfg := validEmailConfig()
	cfg.SMTPUsername = "user"

	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "configured together") {
		t.Fatalf("validate error = %v, want incomplete credential error", err)
	}
}

func TestEmailConfigRequiresLeaseLongerThanSendTimeout(t *testing.T) {
	cfg := validEmailConfig()
	cfg.ProcessingStaleAfter = cfg.SMTPTimeout

	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "must be greater") {
		t.Fatalf("validate error = %v, want stale-after constraint", err)
	}
}

func TestEmailIsDeliverable(t *testing.T) {
	for _, test := range []struct {
		provider string
		want     bool
	}{
		{provider: "noop", want: false},
		{provider: " SMTP ", want: true},
	} {
		t.Run(strings.TrimSpace(test.provider), func(t *testing.T) {
			if got := (Email{Provider: test.provider}).IsDeliverable(); got != test.want {
				t.Fatalf("IsDeliverable() = %t, want %t", got, test.want)
			}
		})
	}
}
