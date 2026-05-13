package ratelimiter

import (
	"context"
	"net"
)

type AuthLimiter interface {
	AllowLoginAttempt(ctx context.Context, ip net.IP, email string) (bool, error)
	AllowSignupAttempt(ctx context.Context, ip net.IP, email string) (bool, error)
	AllowPasswordResetAttempt(ctx context.Context, ip net.IP, email string) (bool, error)
	AllowPasskeyLoginAttempt(ctx context.Context, ip net.IP) (bool, error)
	AllowChallengeVerifyAttempt(ctx context.Context, ip net.IP) (bool, error)
	AllowChallengeResendAttempt(ctx context.Context, ip net.IP) (bool, error)
}
