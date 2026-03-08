package ui

import (
	"context"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
)

func Static(ctx context.Context, name string) string {
	v, ok := httpctx.Assets(ctx)
	if !ok {
		return "/auth/static/" + name
	}
	a, _ := v.(render.Assets)
	return a.Static(name)
}
