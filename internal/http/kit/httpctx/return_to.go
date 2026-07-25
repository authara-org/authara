package httpctx

import "context"

type returnToKeyType struct{}
type defaultReturnToKeyType struct{}

var returnToKey = returnToKeyType{}
var defaultReturnToKey = defaultReturnToKeyType{}

func WithReturnTo(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, returnToKey, path)
}

func WithDefaultReturnTo(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, defaultReturnToKey, path)
}

func ReturnTo(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(returnToKey).(string)
	return v, ok
}

func ReturnToOrDefault(ctx context.Context) string {
	return ReturnToOrManualDefault(ctx, "/")
}

func ReturnToOrManualDefault(ctx context.Context, def string) string {
	if rt, ok := ReturnTo(ctx); ok {
		return rt
	}
	if def == "/" {
		if rt, ok := ctx.Value(defaultReturnToKey).(string); ok && rt != "" {
			return rt
		}
	}
	return def
}
