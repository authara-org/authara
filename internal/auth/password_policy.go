package auth

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	DefaultPasswordMinimumLength = 8
	MaximumPasswordLength        = 128
)

var (
	ErrPasswordTooShort        = errors.New("password is too short")
	ErrPasswordTooLong         = errors.New("password is too long")
	ErrPasswordInvalidEncoding = errors.New("password is not valid UTF-8")
)

func passwordMinimumLength(minimum int) int {
	if minimum < DefaultPasswordMinimumLength || minimum > MaximumPasswordLength {
		return DefaultPasswordMinimumLength
	}
	return minimum
}

func validatePassword(password string, minimum int) error {
	if !utf8.ValidString(password) {
		return ErrPasswordInvalidEncoding
	}
	length := utf8.RuneCountInString(password)
	if length < minimum {
		return ErrPasswordTooShort
	}
	if length > MaximumPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}

func PasswordPolicyMessage(err error, minimum int) (string, bool) {
	switch {
	case errors.Is(err, ErrPasswordTooShort):
		return fmt.Sprintf("Password must contain at least %d characters.", minimum), true
	case errors.Is(err, ErrPasswordTooLong):
		return fmt.Sprintf("Password must contain no more than %d characters.", MaximumPasswordLength), true
	case errors.Is(err, ErrPasswordInvalidEncoding):
		return "Password contains invalid characters.", true
	default:
		return "", false
	}
}

func (s *Service) PasswordMinimumLength() int {
	return passwordMinimumLength(s.passwordMinimumLength)
}

func (s *Service) ValidatePassword(_ context.Context, password string) error {
	return validatePassword(password, s.PasswordMinimumLength())
}

func (s *Service) HashPassword(ctx context.Context, password string) (string, error) {
	if err := s.ValidatePassword(ctx, password); err != nil {
		return "", err
	}
	return hashPassword(password)
}
