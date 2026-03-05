package ratelimiter

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

func IsRateLimited(err error) (*RateLimitedError, bool) {
	var rl *RateLimitedError
	if errors.As(err, &rl) {
		return rl, true
	}
	return nil, false
}

type counter struct {
	count int
	reset time.Time
}

type InMemoryLimiter struct {
	mu sync.Mutex

	loginByIP    map[string]*counter
	loginByEmail map[string]*counter

	signupByIP    map[string]*counter
	signupByEmail map[string]*counter

	loginIPLimit     int
	loginIPWindow    time.Duration
	loginEmailLimit  int
	loginEmailWindow time.Duration

	signupIPLimit     int
	signupIPWindow    time.Duration
	signupEmailLimit  int
	signupEmailWindow time.Duration

	cleanupEvery int              // sweep every N calls
	callCount    int              // increments each Allow* call
	maxEntries   int              // hard cap to avoid runaway memory
	now          func() time.Time // for tests
}

type LimiterConfig struct {
	LoginIPLimit     int
	LoginIPWindow    time.Duration
	LoginEmailLimit  int
	LoginEmailWindow time.Duration

	SignupIPLimit     int
	SignupIPWindow    time.Duration
	SignupEmailLimit  int
	SignupEmailWindow time.Duration

	CleanupEvery int
	MaxEntries   int
}

func NewInMemoryLimiter(cfg LimiterConfig) AuthLimiter {
	return &InMemoryLimiter{
		loginByIP:     make(map[string]*counter),
		loginByEmail:  make(map[string]*counter),
		signupByIP:    make(map[string]*counter),
		signupByEmail: make(map[string]*counter),

		loginIPLimit:     cfg.LoginIPLimit,
		loginIPWindow:    cfg.LoginIPWindow,
		loginEmailLimit:  cfg.LoginEmailLimit,
		loginEmailWindow: cfg.LoginEmailWindow,

		signupIPLimit:     cfg.SignupIPLimit,
		signupIPWindow:    cfg.SignupIPWindow,
		signupEmailLimit:  cfg.SignupEmailLimit,
		signupEmailWindow: cfg.SignupEmailWindow,

		cleanupEvery: cfg.CleanupEvery,
		maxEntries:   cfg.MaxEntries,
		now:          time.Now,
	}
}

func (l *InMemoryLimiter) AllowLoginAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ip, email,
		l.loginByIP, l.loginByEmail,
		l.loginIPLimit, l.loginIPWindow,
		l.loginEmailLimit, l.loginEmailWindow,
		"login",
	)
}

func (l *InMemoryLimiter) AllowSignupAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ip, email,
		l.signupByIP, l.signupByEmail,
		l.signupIPLimit, l.signupIPWindow,
		l.signupEmailLimit, l.signupEmailWindow,
		"signup",
	)
}

func (l *InMemoryLimiter) allow(
	ip net.IP,
	email string,
	byIP map[string]*counter,
	byEmail map[string]*counter,
	ipLimit int,
	ipWindow time.Duration,
	emailLimit int,
	emailWindow time.Duration,
	kind string,
) (bool, error) {
	now := l.now()

	ipKey := normalizeIP(ip)
	emailKey := normalizeEmail(email)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.callCount++
	if l.callCount%l.cleanupEvery == 0 {
		l.sweepExpiredLocked(now)
		l.enforceMaxEntriesLocked()
	}

	ipCounter := getCounterLocked(byIP, ipKey, now, ipWindow)
	if ipCounter.count >= ipLimit {
		return false, &RateLimitedError{
			RetryAfter: retryAfter(now, ipCounter.reset),
			Scope:      kind + ":ip",
		}
	}
	emailCounter := getCounterLocked(byEmail, emailKey, now, emailWindow)
	if emailCounter.count >= emailLimit {
		return false, &RateLimitedError{
			RetryAfter: retryAfter(now, emailCounter.reset),
			Scope:      kind + ":email",
		}
	}

	ipCounter.count++
	emailCounter.count++

	return true, nil
}

func normalizeEmail(email string) string {
	e := strings.TrimSpace(email)
	if e == "" {
		return "__empty_email__"
	}
	return strings.ToLower(e)
}

func normalizeIP(ip net.IP) string {
	if ip == nil {
		return "__unknown_ip__"
	}

	s := ip.String()
	if s == "" {
		return "__unknown_ip__"
	}
	return s
}

func retryAfter(now, reset time.Time) time.Duration {
	if reset.After(now) {
		return reset.Sub(now)
	}
	return 0
}

func getCounterLocked(m map[string]*counter, key string, now time.Time, window time.Duration) *counter {
	c, ok := m[key]
	if !ok || !now.Before(c.reset) {
		c = &counter{
			count: 0,
			reset: now.Add(window),
		}
		m[key] = c
	}
	return c
}

func (l *InMemoryLimiter) sweepExpiredLocked(now time.Time) {
	sweep := func(m map[string]*counter) {
		for k, c := range m {
			if !now.Before(c.reset) {
				delete(m, k)
			}
		}
	}

	sweep(l.loginByIP)
	sweep(l.loginByEmail)
	sweep(l.signupByIP)
	sweep(l.signupByEmail)
}

func (l *InMemoryLimiter) enforceMaxEntriesLocked() {
	total := len(l.loginByIP) + len(l.loginByEmail) + len(l.signupByIP) + len(l.signupByEmail)
	if total <= l.maxEntries {
		return
	}

	clearMap := func(m map[string]*counter) {
		for k := range m {
			delete(m, k)
			total--
			if total <= l.maxEntries {
				return
			}
		}
	}

	clearMap(l.loginByIP)
	clearMap(l.signupByIP)
	clearMap(l.loginByEmail)
	clearMap(l.signupByEmail)
}
