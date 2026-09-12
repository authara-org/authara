package ratelimiter

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/authara-org/authara/internal/cache"
)

type CacheLimiter struct {
	cache cache.Counter
	cfg   ConfigProvider
}

func NewCacheLimiter(c cache.Counter, cfg LimiterConfig) AuthLimiter {
	return NewCacheLimiterWithConfig(c, func() LimiterConfig { return cfg })
}

func NewCacheLimiterWithConfig(c cache.Counter, provider ConfigProvider) AuthLimiter {
	return &CacheLimiter{
		cache: c,
		cfg:   normalizedConfigProvider(provider),
	}
}

func (l *CacheLimiter) AllowLoginAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	cfg := l.cfg()
	return l.allow(ctx, ip, email,
		cfg.LoginIPLimit, cfg.LoginIPWindow,
		cfg.LoginEmailLimit, cfg.LoginEmailWindow,
		"login", "email",
	)
}

func (l *CacheLimiter) AllowSignupAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	cfg := l.cfg()
	return l.allow(ctx, ip, email,
		cfg.SignupIPLimit, cfg.SignupIPWindow,
		cfg.SignupEmailLimit, cfg.SignupEmailWindow,
		"signup", "email",
	)
}

func (l *CacheLimiter) AllowPasswordResetAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	cfg := l.cfg()
	return l.allow(ctx, ip, email,
		cfg.PasswordResetIPLimit, cfg.PasswordResetIPWindow,
		cfg.PasswordResetEmailLimit, cfg.PasswordResetEmailWindow,
		"password_reset", "email",
	)
}

func (l *CacheLimiter) AllowPasskeyLoginAttempt(ctx context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ctx,
		ip,
		cfg.PasskeyLoginIPLimit,
		cfg.PasskeyLoginIPWindow,
		"passkey_login",
	)
}

func (l *CacheLimiter) AllowPasskeyLoginFinishAttempt(ctx context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ctx,
		ip,
		cfg.PasskeyLoginIPLimit,
		cfg.PasskeyLoginIPWindow,
		"passkey_login_finish",
	)
}

func (l *CacheLimiter) AllowChallengeVerifyAttempt(ctx context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ctx,
		ip,
		cfg.ChallengeVerifyIPLimit,
		cfg.ChallengeVerifyIPWindow,
		"challenge_verify",
	)
}

func (l *CacheLimiter) AllowChallengeResendAttempt(ctx context.Context, ip net.IP) (bool, error) {
	cfg := l.cfg()
	return l.allowIP(
		ctx,
		ip,
		cfg.ChallengeResendIPLimit,
		cfg.ChallengeResendIPWindow,
		"challenge_resend",
	)
}

func (l *CacheLimiter) allowIP(
	ctx context.Context,
	ip net.IP,
	ipLimit int,
	ipWindow time.Duration,
	kind string,
) (bool, error) {
	ipKey := cache.RateLimitKey(kind, "ip", normalizeIP(ip))
	ipCount, ipTTL, err := l.cache.Increment(ctx, ipKey, ipWindow)
	if err != nil {
		return false, fmt.Errorf("rate limit %s ip: %w", kind, err)
	}
	if ipCount > int64(ipLimit) {
		return false, &RateLimitedError{
			RetryAfter: ipTTL,
			Scope:      kind + ":ip",
		}
	}

	return true, nil
}

func (l *CacheLimiter) allow(
	ctx context.Context,
	ip net.IP,
	key string,
	ipLimit int,
	ipWindow time.Duration,
	keyLimit int,
	keyWindow time.Duration,
	kind string,
	keyScope string,
) (bool, error) {
	ipKey := cache.RateLimitKey(kind, "ip", normalizeIP(ip))
	ipCount, ipTTL, err := l.cache.Increment(ctx, ipKey, ipWindow)
	if err != nil {
		return false, fmt.Errorf("rate limit %s ip: %w", kind, err)
	}
	if ipCount > int64(ipLimit) {
		return false, &RateLimitedError{
			RetryAfter: ipTTL,
			Scope:      kind + ":ip",
		}
	}

	normalizedKey := normalizeKey(key)
	limitKey := cache.RateLimitKey(kind, keyScope, normalizedKey)
	keyCount, keyTTL, err := l.cache.Increment(ctx, limitKey, keyWindow)
	if err != nil {
		return false, fmt.Errorf("rate limit %s %s: %w", kind, keyScope, err)
	}
	if keyCount > int64(keyLimit) {
		return false, &RateLimitedError{
			RetryAfter: keyTTL,
			Scope:      kind + ":" + keyScope,
		}
	}

	return true, nil
}
