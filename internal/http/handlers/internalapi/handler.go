package internalapi

import "github.com/authara-org/authara/internal/organization"

type Handler struct {
	Organizations                       *organization.Service
	PublicOrganizationManagementEnabled bool
}

func New(organizations *organization.Service, publicOrganizationManagementEnabled bool) *Handler {
	return &Handler{
		Organizations:                       organizations,
		PublicOrganizationManagementEnabled: publicOrganizationManagementEnabled,
	}
}
