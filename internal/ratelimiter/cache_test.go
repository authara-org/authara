package ratelimiter

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestCacheLimiterRateLimitsByIP(t *testing.T) {
	limiter := NewCacheLimiter(newFakeCounterCache(), LimiterConfig{
		LoginIPLimit:     2,
		LoginIPWindow:    time.Minute,
		LoginEmailLimit:  99,
		LoginEmailWindow: time.Minute,
	})

	ip := net.ParseIP("192.0.2.1")
	assertLoginAllowed(t, limiter, ip, "a@example.com")
	assertLoginAllowed(t, limiter, ip, "b@example.com")

	allowed, err := limiter.AllowLoginAttempt(context.Background(), ip, "c@example.com")
	assertRateLimited(t, allowed, err, "login:ip")
}

func TestCacheLimiterRateLimitsByKey(t *testing.T) {
	limiter := NewCacheLimiter(newFakeCounterCache(), LimiterConfig{
		LoginIPLimit:     99,
		LoginIPWindow:    time.Minute,
		LoginEmailLimit:  2,
		LoginEmailWindow: time.Minute,
	})

	email := "user@example.com"
	assertLoginAllowed(t, limiter, net.ParseIP("192.0.2.1"), email)
	assertLoginAllowed(t, limiter, net.ParseIP("192.0.2.2"), email)

	allowed, err := limiter.AllowLoginAttempt(context.Background(), net.ParseIP("192.0.2.3"), email)
	assertRateLimited(t, allowed, err, "login:email")
}

func TestCacheLimiterAllowsAfterWindowExpires(t *testing.T) {
	c := newFakeCounterCache()
	limiter := NewCacheLimiter(c, LimiterConfig{
		LoginIPLimit:     1,
		LoginIPWindow:    time.Minute,
		LoginEmailLimit:  99,
		LoginEmailWindow: time.Minute,
	})

	ip := net.ParseIP("192.0.2.1")
	assertLoginAllowed(t, limiter, ip, "a@example.com")

	allowed, err := limiter.AllowLoginAttempt(context.Background(), ip, "b@example.com")
	assertRateLimited(t, allowed, err, "login:ip")

	c.advance(time.Minute + time.Second)

	assertLoginAllowed(t, limiter, ip, "c@example.com")
}

func TestCacheLimiterChallengeVerifyIsIPOnly(t *testing.T) {
	limiter := NewCacheLimiter(newFakeCounterCache(), LimiterConfig{
		ChallengeVerifyIPLimit:  2,
		ChallengeVerifyIPWindow: time.Minute,
	})

	ip := net.ParseIP("192.0.2.1")
	assertChallengeVerifyAllowed(t, limiter, ip)
	assertChallengeVerifyAllowed(t, limiter, ip)

	allowed, err := limiter.AllowChallengeVerifyAttempt(context.Background(), ip)
	assertRateLimited(t, allowed, err, "challenge_verify:ip")
}

func assertLoginAllowed(t *testing.T, limiter AuthLimiter, ip net.IP, email string) {
	t.Helper()

	allowed, err := limiter.AllowLoginAttempt(context.Background(), ip, email)
	assertAllowed(t, allowed, err)
}

func assertChallengeVerifyAllowed(t *testing.T, limiter AuthLimiter, ip net.IP) {
	t.Helper()

	allowed, err := limiter.AllowChallengeVerifyAttempt(context.Background(), ip)
	assertAllowed(t, allowed, err)
}

func assertAllowed(t *testing.T, allowed bool, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("allow failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected request to be allowed")
	}
}

func assertRateLimited(t *testing.T, allowed bool, err error, scope string) {
	t.Helper()

	if allowed {
		t.Fatal("expected request to be rate limited")
	}

	rl, ok := IsRateLimited(err)
	if !ok {
		t.Fatalf("expected RateLimitedError, got %T %v", err, err)
	}
	if rl.Scope != scope {
		t.Fatalf("scope mismatch: got %q want %q", rl.Scope, scope)
	}
	if rl.RetryAfter <= 0 {
		t.Fatalf("expected positive retry after, got %s", rl.RetryAfter)
	}
}

type fakeCounterCache struct {
	mu       sync.Mutex
	now      time.Time
	counters map[string]fakeCounter
}

type fakeCounter struct {
	count   int64
	expires time.Time
}

func newFakeCounterCache() *fakeCounterCache {
	return &fakeCounterCache{
		now:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		counters: make(map[string]fakeCounter),
	}
}

func (c *fakeCounterCache) Increment(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (int64, time.Duration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	counter, ok := c.counters[key]
	if !ok || !c.now.Before(counter.expires) {
		counter = fakeCounter{expires: c.now.Add(ttl)}
	}
	counter.count++
	c.counters[key] = counter

	return counter.count, counter.expires.Sub(c.now), nil
}

func (c *fakeCounterCache) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}
