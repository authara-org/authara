package cache

import "fmt"

func AutharaUserKey(userID string) string {
	return fmt.Sprintf("authara:user:%s", userID)
}

func RateLimitKey(kind, scope, value string) string {
	return fmt.Sprintf("authara:ratelimit:%s:%s:%s", kind, scope, value)
}
