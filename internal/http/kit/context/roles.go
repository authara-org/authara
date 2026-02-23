package httpcontext

import (
	"context"

	"github.com/alexlup06-authgate/authgate/internal/session/roles"
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
