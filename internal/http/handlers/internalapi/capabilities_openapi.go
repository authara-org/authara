package internalapi

import (
	"context"

	contract "github.com/authara-org/authara/internal/http/openapi"
)

func (h *Handler) GetPublicCapabilities(ctx context.Context, _ contract.GetPublicCapabilitiesRequestObject) (contract.GetPublicCapabilitiesResponseObject, error) {
	return h.capabilities(), nil
}

func (h *Handler) capabilities() contract.GetPublicCapabilitiesResponseObject {
	mode := h.Organizations.Mode()
	return contract.GetPublicCapabilities200JSONResponse(contract.Capabilities{
		OrganizationMode:                   contract.CapabilitiesOrganizationMode(mode),
		HasVisibleOrganizations:            mode.HasVisibleOrganizations(),
		AllowsInvitations:                  mode.AllowsInvitations(),
		AllowsPublicOrganizationManagement: h.PublicOrganizationManagementEnabled,
		AllowsOrgSwitching:                 mode.AllowsOrgSwitching(),
		AllowsUserCreatedTeamOrgs:          mode.AllowsUserCreatedTeamOrgs(),
		AllowsOrganizationLeave:            mode.AllowsLeaveOrg(),
	})
}
