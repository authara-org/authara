package organization

import "github.com/authara-org/authara/internal/domain"

type OrgMode string

const (
	OrgModePersonal OrgMode = "personal"
	OrgModeSingle   OrgMode = "single"
	OrgModeMulti    OrgMode = "multi"
)

type SignupSource string

const (
	SignupSourceDirect SignupSource = "direct"
	SignupSourceInvite SignupSource = "invite"
)

type SignupOrganizationPlan struct {
	CreateInitialOrg bool
	InitialOrgKind   domain.OrganizationKind
	AcceptInvitation bool
}

func SignupOrganizationPlanFor(mode OrgMode, source SignupSource) (SignupOrganizationPlan, error) {
	switch mode {
	case OrgModePersonal:
		if source == SignupSourceInvite {
			return SignupOrganizationPlan{}, ErrOrganizationInviteForbidden
		}
		return SignupOrganizationPlan{CreateInitialOrg: true, InitialOrgKind: domain.OrganizationKindPersonal}, nil
	case OrgModeSingle:
		if source == SignupSourceInvite {
			return SignupOrganizationPlan{AcceptInvitation: true}, nil
		}
		return SignupOrganizationPlan{CreateInitialOrg: true, InitialOrgKind: domain.OrganizationKindTeam}, nil
	case OrgModeMulti:
		return SignupOrganizationPlan{
			CreateInitialOrg: true,
			InitialOrgKind:   domain.OrganizationKindPersonal,
			AcceptInvitation: source == SignupSourceInvite,
		}, nil
	default:
		return SignupOrganizationPlan{}, ErrInvalidOrganizationMode
	}
}

func (m OrgMode) AllowsInvitations() bool {
	return m == OrgModeSingle || m == OrgModeMulti
}

func (m OrgMode) AllowsOrgSwitching() bool {
	return m == OrgModeMulti
}

func (m OrgMode) AllowsUserCreatedTeamOrgs() bool {
	return m == OrgModeMulti
}

func (m OrgMode) AllowsLeaveOrg() bool {
	return m == OrgModeMulti
}

func (m OrgMode) HasVisibleOrganizations() bool {
	return m == OrgModeSingle || m == OrgModeMulti
}
