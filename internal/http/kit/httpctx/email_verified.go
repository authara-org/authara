package httpctx

import "context"

type emailVerifiedKeyType struct{}

var emailVerifiedKey emailVerifiedKeyType

func WithEmailVerified(ctx context.Context, verified bool) context.Context {
	return context.WithValue(ctx, emailVerifiedKey, verified)
}

func EmailVerified(ctx context.Context) (bool, bool) {
	verified, ok := ctx.Value(emailVerifiedKey).(bool)
	return verified, ok
}
