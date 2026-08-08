package internalapi

import (
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/organization"
)

type Handler struct {
	Auth                                *auth.Service
	Organizations                       *organization.Service
	PublicOrganizationManagementEnabled bool
}

func New(authService *auth.Service, organizations *organization.Service, publicOrganizationManagementEnabled bool) *Handler {
	return &Handler{
		Auth:                                authService,
		Organizations:                       organizations,
		PublicOrganizationManagementEnabled: publicOrganizationManagementEnabled,
	}
}
