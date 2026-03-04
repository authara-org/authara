package ui

import (
	"context"

	"github.com/alexlup06-authgate/authgate/internal/http/kit/httpctx"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/render"
)

func Static(ctx context.Context, name string) string {
	v, ok := httpctx.Assets(ctx)
	if !ok {
		return "/auth/static/" + name
	}
	a, _ := v.(render.Assets)
	return a.Static(name)
}
