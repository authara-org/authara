package httpctx

import (
	"context"

	"github.com/authara-org/authara/internal/session/roles"
)

type rolesKeyType struct{}

var rolesKey = rolesKeyType{}

func WithRoles(ctx context.Context, roles roles.Roles) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}

func Roles(ctx context.Context) (roles.Roles, bool) {
	roles, ok := ctx.Value(rolesKey).(roles.Roles)
	return roles, ok
}
