package ratelimiter

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestInMemoryLimiterReadsLiveConfigForEachAttempt(t *testing.T) {
	cfg := LimiterConfig{
		LoginIPLimit: 3, LoginIPWindow: time.Minute,
		LoginEmailLimit: 99, LoginEmailWindow: time.Minute,
	}
	reads := 0
	limiter := NewInMemoryLimiterWithConfig(func() LimiterConfig {
		reads++
		return cfg
	})
	ip := net.ParseIP("192.0.2.10")
	assertLoginAllowed(t, limiter, ip, "first@example.com")
	assertLoginAllowed(t, limiter, ip, "second@example.com")
	cfg.LoginIPLimit = 2

	allowed, err := limiter.AllowLoginAttempt(context.Background(), ip, "third@example.com")
	assertRateLimited(t, allowed, err, "login:ip")
	if reads != 3 {
		t.Fatalf("config reads = %d, want one per attempt", reads)
	}
}

func TestCacheLimiterReadsLiveConfigForEachAttempt(t *testing.T) {
	cfg := LimiterConfig{
		LoginIPLimit: 3, LoginIPWindow: time.Minute,
		LoginEmailLimit: 99, LoginEmailWindow: time.Minute,
	}
	reads := 0
	limiter := NewCacheLimiterWithConfig(newFakeCounterCache(), func() LimiterConfig {
		reads++
		return cfg
	})
	ip := net.ParseIP("192.0.2.20")
	assertLoginAllowed(t, limiter, ip, "first@example.com")
	assertLoginAllowed(t, limiter, ip, "second@example.com")
	cfg.LoginIPLimit = 2

	allowed, err := limiter.AllowLoginAttempt(context.Background(), ip, "third@example.com")
	assertRateLimited(t, allowed, err, "login:ip")
	if reads != 3 {
		t.Fatalf("config reads = %d, want one per attempt", reads)
	}
}
