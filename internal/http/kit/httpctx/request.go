package httpctx

import "context"

type htmxKeyType struct{}
type csrfKeyType struct{}

var (
	htmxKey = htmxKeyType{}
	csrfKey = csrfKeyType{}
)

func WithHTMX(ctx context.Context) context.Context {
	return context.WithValue(ctx, htmxKey, true)
}

func IsHTMX(ctx context.Context) bool {
	v, ok := ctx.Value(htmxKey).(bool)
	return ok && v
}

func WithCSRF(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey, token)
}

func CSRFToken(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(csrfKey).(string)
	return v, ok
}
