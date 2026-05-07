package ratelimiter

import "time"

// RateLimitedError is returned when the caller should reject the attempt.
// RetryAfter can be used to set HTTP Retry-After header or show a UI message.
type RateLimitedError struct {
	RetryAfter time.Duration
	Scope      string
}

func (e *RateLimitedError) Error() string {
	return "rate limited"
}
