package httpctx

import (
	"context"

	"github.com/authara-org/authara/internal/http/kit/flash"
)

type flashKeyType struct{}

var flashKey = flashKeyType{}

func WithFlash(ctx context.Context, f *flash.Message) context.Context {
	return context.WithValue(ctx, flashKey, f)
}

func Flash(ctx context.Context) (*flash.Message, bool) {
	f, ok := ctx.Value(flashKey).(*flash.Message)
	return f, ok
}
