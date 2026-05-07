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

	passwordResetByIP    map[string]*counter
	passwordResetByEmail map[string]*counter

	challengeVerifyByIP map[string]*counter
	challengeVerifyByID map[string]*counter

	challengeResendByIP map[string]*counter
	challengeResendByID map[string]*counter

	loginIPLimit     int
	loginIPWindow    time.Duration
	loginEmailLimit  int
	loginEmailWindow time.Duration

	signupIPLimit     int
	signupIPWindow    time.Duration
	signupEmailLimit  int
	signupEmailWindow time.Duration

	passwordResetIPLimit     int
	passwordResetIPWindow    time.Duration
	passwordResetEmailLimit  int
	passwordResetEmailWindow time.Duration

	challengeVerifyIPLimit  int
	challengeVerifyIPWindow time.Duration
	challengeVerifyIDLimit  int
	challengeVerifyIDWindow time.Duration

	challengeResendIPLimit  int
	challengeResendIPWindow time.Duration
	challengeResendIDLimit  int
	challengeResendIDWindow time.Duration

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

	PasswordResetIPLimit     int
	PasswordResetIPWindow    time.Duration
	PasswordResetEmailLimit  int
	PasswordResetEmailWindow time.Duration

	ChallengeVerifyIPLimit  int
	ChallengeVerifyIPWindow time.Duration
	ChallengeVerifyIDLimit  int
	ChallengeVerifyIDWindow time.Duration

	ChallengeResendIPLimit  int
	ChallengeResendIPWindow time.Duration
	ChallengeResendIDLimit  int
	ChallengeResendIDWindow time.Duration

	CleanupEvery int
	MaxEntries   int
}

func NewInMemoryLimiter(cfg LimiterConfig) AuthLimiter {
	cfg = defaultLimiterConfig(cfg)

	return &InMemoryLimiter{
		loginByIP:            make(map[string]*counter),
		loginByEmail:         make(map[string]*counter),
		signupByIP:           make(map[string]*counter),
		signupByEmail:        make(map[string]*counter),
		passwordResetByIP:    make(map[string]*counter),
		passwordResetByEmail: make(map[string]*counter),
		challengeVerifyByIP:  make(map[string]*counter),
		challengeVerifyByID:  make(map[string]*counter),
		challengeResendByIP:  make(map[string]*counter),
		challengeResendByID:  make(map[string]*counter),

		loginIPLimit:     cfg.LoginIPLimit,
		loginIPWindow:    cfg.LoginIPWindow,
		loginEmailLimit:  cfg.LoginEmailLimit,
		loginEmailWindow: cfg.LoginEmailWindow,

		signupIPLimit:     cfg.SignupIPLimit,
		signupIPWindow:    cfg.SignupIPWindow,
		signupEmailLimit:  cfg.SignupEmailLimit,
		signupEmailWindow: cfg.SignupEmailWindow,

		passwordResetIPLimit:     cfg.PasswordResetIPLimit,
		passwordResetIPWindow:    cfg.PasswordResetIPWindow,
		passwordResetEmailLimit:  cfg.PasswordResetEmailLimit,
		passwordResetEmailWindow: cfg.PasswordResetEmailWindow,

		challengeVerifyIPLimit:  cfg.ChallengeVerifyIPLimit,
		challengeVerifyIPWindow: cfg.ChallengeVerifyIPWindow,
		challengeVerifyIDLimit:  cfg.ChallengeVerifyIDLimit,
		challengeVerifyIDWindow: cfg.ChallengeVerifyIDWindow,

		challengeResendIPLimit:  cfg.ChallengeResendIPLimit,
		challengeResendIPWindow: cfg.ChallengeResendIPWindow,
		challengeResendIDLimit:  cfg.ChallengeResendIDLimit,
		challengeResendIDWindow: cfg.ChallengeResendIDWindow,

		cleanupEvery: cfg.CleanupEvery,
		maxEntries:   cfg.MaxEntries,
		now:          time.Now,
	}
}

func defaultLimiterConfig(cfg LimiterConfig) LimiterConfig {
	setIntDefault(&cfg.LoginIPLimit, 5)
	setDurationDefault(&cfg.LoginIPWindow, time.Minute)
	setIntDefault(&cfg.LoginEmailLimit, 10)
	setDurationDefault(&cfg.LoginEmailWindow, time.Hour)

	setIntDefault(&cfg.SignupIPLimit, 3)
	setDurationDefault(&cfg.SignupIPWindow, time.Hour)
	setIntDefault(&cfg.SignupEmailLimit, 3)
	setDurationDefault(&cfg.SignupEmailWindow, 24*time.Hour)

	setIntDefault(&cfg.PasswordResetIPLimit, 5)
	setDurationDefault(&cfg.PasswordResetIPWindow, time.Hour)
	setIntDefault(&cfg.PasswordResetEmailLimit, 3)
	setDurationDefault(&cfg.PasswordResetEmailWindow, 24*time.Hour)

	setIntDefault(&cfg.ChallengeVerifyIPLimit, 30)
	setDurationDefault(&cfg.ChallengeVerifyIPWindow, 10*time.Minute)
	setIntDefault(&cfg.ChallengeVerifyIDLimit, 10)
	setDurationDefault(&cfg.ChallengeVerifyIDWindow, 30*time.Minute)

	setIntDefault(&cfg.ChallengeResendIPLimit, 10)
	setDurationDefault(&cfg.ChallengeResendIPWindow, time.Hour)
	setIntDefault(&cfg.ChallengeResendIDLimit, 5)
	setDurationDefault(&cfg.ChallengeResendIDWindow, 30*time.Minute)

	setIntDefault(&cfg.CleanupEvery, 200)
	setIntDefault(&cfg.MaxEntries, 50000)

	return cfg
}

func setIntDefault(v *int, def int) {
	if *v <= 0 {
		*v = def
	}
}

func setDurationDefault(v *time.Duration, def time.Duration) {
	if *v <= 0 {
		*v = def
	}
}

func (l *InMemoryLimiter) AllowLoginAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ip, email,
		l.loginByIP, l.loginByEmail,
		l.loginIPLimit, l.loginIPWindow,
		l.loginEmailLimit, l.loginEmailWindow,
		"login", "email",
	)
}

func (l *InMemoryLimiter) AllowSignupAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ip, email,
		l.signupByIP, l.signupByEmail,
		l.signupIPLimit, l.signupIPWindow,
		l.signupEmailLimit, l.signupEmailWindow,
		"signup", "email",
	)
}

func (l *InMemoryLimiter) AllowPasswordResetAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ip, email,
		l.passwordResetByIP, l.passwordResetByEmail,
		l.passwordResetIPLimit, l.passwordResetIPWindow,
		l.passwordResetEmailLimit, l.passwordResetEmailWindow,
		"password_reset", "email",
	)
}

func (l *InMemoryLimiter) AllowChallengeVerifyAttempt(_ context.Context, ip net.IP, challengeID string) (bool, error) {
	return l.allow(ip, challengeID,
		l.challengeVerifyByIP, l.challengeVerifyByID,
		l.challengeVerifyIPLimit, l.challengeVerifyIPWindow,
		l.challengeVerifyIDLimit, l.challengeVerifyIDWindow,
		"challenge_verify", "challenge",
	)
}

func (l *InMemoryLimiter) AllowChallengeResendAttempt(_ context.Context, ip net.IP, challengeID string) (bool, error) {
	return l.allow(ip, challengeID,
		l.challengeResendByIP, l.challengeResendByID,
		l.challengeResendIPLimit, l.challengeResendIPWindow,
		l.challengeResendIDLimit, l.challengeResendIDWindow,
		"challenge_resend", "challenge",
	)
}

func (l *InMemoryLimiter) allow(
	ip net.IP,
	key string,
	byIP map[string]*counter,
	byKey map[string]*counter,
	ipLimit int,
	ipWindow time.Duration,
	keyLimit int,
	keyWindow time.Duration,
	kind string,
	keyScope string,
) (bool, error) {
	now := l.now()

	ipKey := normalizeIP(ip)
	normalizedKey := normalizeKey(key)

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
	keyCounter := getCounterLocked(byKey, normalizedKey, now, keyWindow)
	if keyCounter.count >= keyLimit {
		return false, &RateLimitedError{
			RetryAfter: retryAfter(now, keyCounter.reset),
			Scope:      kind + ":" + keyScope,
		}
	}

	ipCounter.count++
	keyCounter.count++

	return true, nil
}

func normalizeKey(key string) string {
	k := strings.TrimSpace(key)
	if k == "" {
		return "__empty_key__"
	}
	return strings.ToLower(k)
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
	sweep(l.passwordResetByIP)
	sweep(l.passwordResetByEmail)
	sweep(l.challengeVerifyByIP)
	sweep(l.challengeVerifyByID)
	sweep(l.challengeResendByIP)
	sweep(l.challengeResendByID)
}

func (l *InMemoryLimiter) enforceMaxEntriesLocked() {
	total := len(l.loginByIP) + len(l.loginByEmail) +
		len(l.signupByIP) + len(l.signupByEmail) +
		len(l.passwordResetByIP) + len(l.passwordResetByEmail) +
		len(l.challengeVerifyByIP) + len(l.challengeVerifyByID) +
		len(l.challengeResendByIP) + len(l.challengeResendByID)
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
	clearMap(l.passwordResetByIP)
	clearMap(l.challengeVerifyByIP)
	clearMap(l.challengeResendByIP)
	clearMap(l.loginByEmail)
	clearMap(l.signupByEmail)
	clearMap(l.passwordResetByEmail)
	clearMap(l.challengeVerifyByID)
	clearMap(l.challengeResendByID)
}
