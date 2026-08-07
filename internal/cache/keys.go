package cache

import "fmt"

func AutharaUserKey(userID string) string {
	return fmt.Sprintf("authara:user:%s", userID)
}

func RateLimitKey(kind, scope, value string) string {
	return fmt.Sprintf("authara:ratelimit:%s:%s:%s", kind, scope, value)
}

func RevokedAccessTokenKey(tokenHash string) string {
	return fmt.Sprintf("authara:access-token:revoked:token:%s", tokenHash)
}

func RevokedAccessTokenSessionKey(sessionID string) string {
	return fmt.Sprintf("authara:access-token:revoked:session:%s", sessionID)
}

func RevokedAccessTokenUserKey(userID string) string {
	return fmt.Sprintf("authara:access-token:revoked:user:%s", userID)
}

func RevokedAccessTokenMembershipKey(userID, organizationID string) string {
	return fmt.Sprintf("authara:access-token:revoked:membership:%s:%s", userID, organizationID)
}
