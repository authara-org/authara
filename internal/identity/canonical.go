package identity

import "strings"

// CanonicalEmail returns the representation Authara uses for email identity.
func CanonicalEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CanonicalUsername returns the representation Authara uses for username identity.
func CanonicalUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
