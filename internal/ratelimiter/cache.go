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
	cfg   LimiterConfig
}

func NewCacheLimiter(c cache.Counter, cfg LimiterConfig) AuthLimiter {
	return &CacheLimiter{
		cache: c,
		cfg:   defaultLimiterConfig(cfg),
	}
}

func (l *CacheLimiter) AllowLoginAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ctx, ip, email,
		l.cfg.LoginIPLimit, l.cfg.LoginIPWindow,
		l.cfg.LoginEmailLimit, l.cfg.LoginEmailWindow,
		"login", "email",
	)
}

func (l *CacheLimiter) AllowSignupAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ctx, ip, email,
		l.cfg.SignupIPLimit, l.cfg.SignupIPWindow,
		l.cfg.SignupEmailLimit, l.cfg.SignupEmailWindow,
		"signup", "email",
	)
}

func (l *CacheLimiter) AllowPasswordResetAttempt(ctx context.Context, ip net.IP, email string) (bool, error) {
	return l.allow(ctx, ip, email,
		l.cfg.PasswordResetIPLimit, l.cfg.PasswordResetIPWindow,
		l.cfg.PasswordResetEmailLimit, l.cfg.PasswordResetEmailWindow,
		"password_reset", "email",
	)
}

func (l *CacheLimiter) AllowChallengeVerifyAttempt(ctx context.Context, ip net.IP) (bool, error) {
	return l.allowIP(
		ctx,
		ip,
		l.cfg.ChallengeVerifyIPLimit,
		l.cfg.ChallengeVerifyIPWindow,
		"challenge_verify",
	)
}

func (l *CacheLimiter) AllowChallengeResendAttempt(ctx context.Context, ip net.IP) (bool, error) {
	return l.allowIP(
		ctx,
		ip,
		l.cfg.ChallengeResendIPLimit,
		l.cfg.ChallengeResendIPWindow,
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
