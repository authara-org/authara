package auth

import "strings"

func IsValidEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return strings.Contains(email, "@")
}

func IsValidPassword(pw string) bool {
	return len(pw) >= 8 && len(pw) <= 128
}
