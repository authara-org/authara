package ratelimit

import (
	"context"
	"net"
)

type AuthLimiter interface {
	AllowLoginAttempt(ctx context.Context, ip net.IP, email string) (bool, error)
	AllowSignupAttempt(ctx context.Context, ip net.IP, email string) (bool, error)
}
