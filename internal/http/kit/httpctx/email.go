package httpctx

import (
	"context"
)

type EmailKeyType struct{}

var emailKey = EmailKeyType{}

func WithEmail(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, emailKey, id)
}

func Email(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(emailKey).(string)
	return id, ok
}

func EmailOrDefault(ctx context.Context, def string) string {
	if rt, ok := Email(ctx); ok {
		return rt
	}
	return def
}
