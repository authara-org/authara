package handlers

import "strings"

func isValidEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	return strings.Contains(email, "@")
}

func isValidPassword(pw string) bool {
	return len(pw) >= 8 && len(pw) <= 128
}
