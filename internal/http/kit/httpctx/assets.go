package httpctx

import (
	"context"
)

type assetsKey struct{}

func WithAssets(ctx context.Context, a any) context.Context {
	return context.WithValue(ctx, assetsKey{}, a)
}

func Assets(ctx context.Context) (any, bool) {
	v := ctx.Value(assetsKey{})
	return v, v != nil
}
