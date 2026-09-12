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
	mu  sync.Mutex
	cfg ConfigProvider

	loginByIP    map[string]*counter
	loginByEmail map[string]*counter

	signupByIP    map[string]*counter
	signupByEmail map[string]*counter

	passwordResetByIP    map[string]*counter
	passwordResetByEmail map[string]*counter

	passkeyLoginByIP       map[string]*counter
	passkeyLoginFinishByIP map[string]*counter

	challengeVerifyByIP map[string]*counter

	challengeResendByIP map[string]*counter

	callCount int              // increments each Allow* call
	now       func() time.Time // for tests
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

	PasskeyLoginIPLimit  int
	PasskeyLoginIPWindow time.Duration

	ChallengeVerifyIPLimit  int
	ChallengeVerifyIPWindow time.Duration

	ChallengeResendIPLimit  int
	ChallengeResendIPWindow time.Duration

	CleanupEvery int
	MaxEntries   int
}

type ConfigProvider func() LimiterConfig

func NewInMemoryLimiter(cfg LimiterConfig) AuthLimiter {
	return NewInMemoryLimiterWithConfig(func() LimiterConfig { return cfg })
}

func NewInMemoryLimiterWithConfig(provider ConfigProvider) AuthLimiter {
	return &InMemoryLimiter{
		cfg:                    normalizedConfigProvider(provider),
		loginByIP:              make(map[string]*counter),
		loginByEmail:           make(map[string]*counter),
		signupByIP:             make(map[string]*counter),
		signupByEmail:          make(map[string]*counter),
		passwordResetByIP:      make(map[string]*counter),
		passwordResetByEmail:   make(map[string]*counter),
		passkeyLoginByIP:       make(map[string]*counter),
		passkeyLoginFinishByIP: make(map[string]*counter),
		challengeVerifyByIP:    make(map[string]*counter),
		challengeResendByIP:    make(map[string]*counter),
		now:                    time.Now,
	}
}

func normalizedConfigProvider(provider ConfigProvider) ConfigProvider {
	if provider == nil {
		provider = func() LimiterConfig { return LimiterConfig{} }
	}
	return func() LimiterConfig { return defaultLimiterConfig(provider()) }
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

	setIntDefault(&cfg.PasskeyLoginIPLimit, 30)
	setDurationDefault(&cfg.PasskeyLoginIPWindow, 10*time.Minute)

	setIntDefault(&cfg.ChallengeVerifyIPLimit, 30)
	setDurationDefault(&cfg.ChallengeVerifyIPWindow, 10*time.Minute)

	setIntDefault(&cfg.ChallengeResendIPLimit, 10)
	setDurationDefault(&cfg.ChallengeResendIPWindow, time.Hour)

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
	cfg := l.cfg()
	return l.allow(ip, email,
		l.loginByIP, l.loginByEmail,
		cfg.LoginIPLimit, cfg.LoginIPWindow,
		cfg.LoginEmailLimit, cfg.LoginEmailWindow,
		cfg.CleanupEvery, cfg.MaxEntries,
		"login", "email",
	)
}

func (l *InMemoryLimiter) AllowSignupAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	cfg := l.cfg()
	return l.allow(ip, email,
		l.signupByIP, l.signupByEmail,
		cfg.SignupIPLimit, cfg.SignupIPWindow,
		cfg.SignupEmailLimit, cfg.SignupEmailWindow,
		cfg.CleanupEvery, cfg.MaxEntries,
		"signup", "email",
	)
}

func (l *InMemoryLimiter) AllowPasswordResetAttempt(_ context.Context, ip net.IP, email string) (bool, error) {
	cfg := l.cfg()
	return l.allow(ip, email,
		l.passwordResetByIP, l.passwordResetByEmail,
		cfg.PasswordResetIPLimit, cfg.PasswordResetIPWindow,
		cfg.PasswordResetEmailLimit, cfg.PasswordResetEmailWindow,
		cfg.CleanupEvery, cfg.MaxEntries,
		"password_reset", "email",
	)
}

func (l *InMemoryLimiter) AllowPasskeyLoginAttempt(_ context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ip,
		l.passkeyLoginByIP,
		cfg.PasskeyLoginIPLimit,
		cfg.PasskeyLoginIPWindow,
		cfg.CleanupEvery,
		cfg.MaxEntries,
		"passkey_login",
	)
}

func (l *InMemoryLimiter) AllowPasskeyLoginFinishAttempt(_ context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ip,
		l.passkeyLoginFinishByIP,
		cfg.PasskeyLoginIPLimit,
		cfg.PasskeyLoginIPWindow,
		cfg.CleanupEvery,
		cfg.MaxEntries,
		"passkey_login_finish",
	)
}

func (l *InMemoryLimiter) AllowChallengeVerifyAttempt(_ context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ip,
		l.challengeVerifyByIP,
		cfg.ChallengeVerifyIPLimit,
		cfg.ChallengeVerifyIPWindow,
		cfg.CleanupEvery,
		cfg.MaxEntries,
		"challenge_verify",
	)
}

func (l *InMemoryLimiter) AllowChallengeResendAttempt(_ context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ip,
		l.challengeResendByIP,
		cfg.ChallengeResendIPLimit,
		cfg.ChallengeResendIPWindow,
		cfg.CleanupEvery,
		cfg.MaxEntries,
		"challenge_resend",
	)
}

func (l *InMemoryLimiter) allowIP(
	ip net.IP,
	byIP map[string]*counter,
	ipLimit int,
	ipWindow time.Duration,
	cleanupEvery int,
	maxEntries int,
	kind string,
) (bool, error) {
	now := l.now()
	ipKey := normalizeIP(ip)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.callCount++
	if l.callCount%cleanupEvery == 0 {
		l.sweepExpiredLocked(now)
		l.enforceMaxEntriesLocked(maxEntries)
	}

	ipCounter := getCounterLocked(byIP, ipKey, now, ipWindow)
	if ipCounter.count >= ipLimit {
		return false, &RateLimitedError{
			RetryAfter: retryAfter(now, ipCounter.reset),
			Scope:      kind + ":ip",
		}
	}

	ipCounter.count++

	return true, nil
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
	cleanupEvery int,
	maxEntries int,
	kind string,
	keyScope string,
) (bool, error) {
	now := l.now()

	ipKey := normalizeIP(ip)
	normalizedKey := normalizeKey(key)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.callCount++
	if l.callCount%cleanupEvery == 0 {
		l.sweepExpiredLocked(now)
		l.enforceMaxEntriesLocked(maxEntries)
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
	sweep(l.passkeyLoginByIP)
	sweep(l.passkeyLoginFinishByIP)
	sweep(l.challengeVerifyByIP)
	sweep(l.challengeResendByIP)
}

func (l *InMemoryLimiter) enforceMaxEntriesLocked(maxEntries int) {
	total := len(l.loginByIP) + len(l.loginByEmail) +
		len(l.signupByIP) + len(l.signupByEmail) +
		len(l.passwordResetByIP) + len(l.passwordResetByEmail) +
		len(l.passkeyLoginByIP) + len(l.passkeyLoginFinishByIP) +
		len(l.challengeVerifyByIP) +
		len(l.challengeResendByIP)
	if total <= maxEntries {
		return
	}

	clearMap := func(m map[string]*counter) {
		for k := range m {
			delete(m, k)
			total--
			if total <= maxEntries {
				return
			}
		}
	}

	clearMap(l.loginByIP)
	clearMap(l.signupByIP)
	clearMap(l.passwordResetByIP)
	clearMap(l.passkeyLoginByIP)
	clearMap(l.passkeyLoginFinishByIP)
	clearMap(l.challengeVerifyByIP)
	clearMap(l.challengeResendByIP)
	clearMap(l.loginByEmail)
	clearMap(l.signupByEmail)
	clearMap(l.passwordResetByEmail)
}
