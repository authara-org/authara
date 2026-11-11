package bootstrap

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestEnsureBootstrapAllowedEmails(t *testing.T) {
	store := &bootstrapAllowedEmailRecorder{}
	emails := []string{"alpha@example.com", "beta@example.com"}

	if err := EnsureBootstrapAllowedEmails(store, emails); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.emails, emails) {
		t.Fatalf("ensured emails = %v, want %v", store.emails, emails)
	}
}

func TestEnsureBootstrapAllowedEmailsStopsWithoutLeakingAddress(t *testing.T) {
	storeFailure := errors.New("database unavailable")
	store := &bootstrapAllowedEmailRecorder{failAt: 2, err: storeFailure}

	err := EnsureBootstrapAllowedEmails(store, []string{"alpha@example.com", "private@example.com"})
	if !errors.Is(err, storeFailure) {
		t.Fatalf("error = %v, want wrapped store failure", err)
	}
	if strings.Contains(err.Error(), "private@example.com") {
		t.Fatalf("error leaked bootstrap address: %v", err)
	}
}

type bootstrapAllowedEmailRecorder struct {
	emails []string
	failAt int
	err    error
}

func (s *bootstrapAllowedEmailRecorder) EnsureAllowedEmail(_ context.Context, email string) error {
	s.emails = append(s.emails, email)
	if s.failAt == len(s.emails) {
		return s.err
	}
	return nil
}
