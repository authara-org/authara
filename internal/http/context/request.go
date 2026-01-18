package context

import "context"

type ctxKey string

const (
	CtxHXRequest ctxKey = "hx-request"
	CtxBlog      ctxKey = "x-blog"
	CtxFilters   ctxKey = "filters"
	CtxFullPath  ctxKey = "full-path"
)

func WithKV(ctx context.Context, key ctxKey, value any) context.Context {
	return context.WithValue(ctx, key, value)
}

func GetKV[T any](ctx context.Context, key ctxKey) (T, bool) {
	v, ok := ctx.Value(key).(T)
	return v, ok
}

func WithHTMX(ctx context.Context) context.Context {
	return context.WithValue(ctx, CtxHXRequest, true)
}

func IsHTMX(ctx context.Context) bool {
	v, ok := GetKV[bool](ctx, CtxHXRequest)
	return ok && v
}
