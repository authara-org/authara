package httpctx

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type organizationIDKeyType struct{}
type organizationRoleKeyType struct{}

var organizationIDKey organizationIDKeyType
var organizationRoleKey organizationRoleKeyType

func WithOrganizationID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, organizationIDKey, id)
}

func OrganizationID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(organizationIDKey).(uuid.UUID)
	return id, ok
}

func WithOrganizationRole(ctx context.Context, role domain.OrganizationRole) context.Context {
	return context.WithValue(ctx, organizationRoleKey, role)
}

func OrganizationRole(ctx context.Context) (domain.OrganizationRole, bool) {
	role, ok := ctx.Value(organizationRoleKey).(domain.OrganizationRole)
	return role, ok
}
