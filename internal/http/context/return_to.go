package httpcontext

import "context"

type returnToKeyType struct{}

var returnToKey = returnToKeyType{}

func WithReturnTo(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, returnToKey, path)
}

func ReturnTo(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(returnToKey).(string)
	return v, ok
}

func ReturnToOrDefault(ctx context.Context, def string) string {
	if rt, ok := ReturnTo(ctx); ok {
		return rt
	}
	return def
}
