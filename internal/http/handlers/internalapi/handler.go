package internalapi

import (
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/organization"
)

type Handler struct {
	Auth                                *auth.Service
	Organizations                       *organization.Service
	PublicOrganizationManagementEnabled bool
	OrganizationPolicy                  config.OrganizationPolicyReader
}

func NewWithPolicy(authService *auth.Service, organizations *organization.Service, policy config.OrganizationPolicyReader) *Handler {
	return &Handler{Auth: authService, Organizations: organizations, OrganizationPolicy: policy}
}

func (h *Handler) publicOrganizationManagementEnabled() bool {
	if h.OrganizationPolicy != nil {
		return h.OrganizationPolicy.CurrentOrganization().PublicManagementEnabled
	}
	return h.PublicOrganizationManagementEnabled
}

func New(authService *auth.Service, organizations *organization.Service, publicOrganizationManagementEnabled bool) *Handler {
	return &Handler{
		Auth:                                authService,
		Organizations:                       organizations,
		PublicOrganizationManagementEnabled: publicOrganizationManagementEnabled,
	}
}
