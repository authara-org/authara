package auth

import (
	"net/mail"
	"strings"
)

func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) == 0 || len(email) > 254 {
		return false
	}
	if strings.ContainsAny(email, "\r\n\t ") {
		return false
	}

	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return addr.Name == "" && strings.EqualFold(addr.Address, email)
}

func IsValidPassword(pw string) bool {
	return len(pw) >= 8 && len(pw) <= 128
}
