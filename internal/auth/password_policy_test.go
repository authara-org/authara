package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServicePasswordPolicyUsesUnicodeLengthAndConfiguredMinimum(t *testing.T) {
	service := New(Config{PasswordMinimumLength: 10})

	if err := service.ValidatePassword(context.Background(), "pässwörd🔐x"); err != nil {
		t.Fatalf("ten-character Unicode password was rejected: %v", err)
	}
	if err := service.ValidatePassword(context.Background(), "pässwörd🔐"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("short Unicode password error = %v", err)
	}
	if got := service.PasswordMinimumLength(); got != 10 {
		t.Fatalf("minimum = %d, want 10", got)
	}
	if err := service.ValidatePassword(context.Background(), strings.Repeat("🔐", MaximumPasswordLength+1)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("long Unicode password error = %v", err)
	}
}

func TestPasswordPolicyHasNoCompositionRules(t *testing.T) {
	service := New(Config{PasswordMinimumLength: 12})
	if err := service.ValidatePassword(context.Background(), "alllowercase"); err != nil {
		t.Fatalf("passphrase without composition variety was rejected: %v", err)
	}
}
