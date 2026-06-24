package organization

import (
	"errors"
	"testing"

	"github.com/authara-org/authara/internal/domain"
)

func TestSignupOrganizationPlanFor(t *testing.T) {
	tests := []struct {
		mode             OrgMode
		source           SignupSource
		createInitialOrg bool
		initialOrgKind   domain.OrganizationKind
		acceptInvitation bool
		wantErr          error
	}{
		{OrgModePersonal, SignupSourceDirect, true, domain.OrganizationKindPersonal, false, nil},
		{OrgModePersonal, SignupSourceInvite, false, "", false, ErrOrganizationInviteForbidden},
		{OrgModeSingle, SignupSourceDirect, true, domain.OrganizationKindTeam, false, nil},
		{OrgModeSingle, SignupSourceInvite, false, "", true, nil},
		{OrgModeMulti, SignupSourceDirect, true, domain.OrganizationKindPersonal, false, nil},
		{OrgModeMulti, SignupSourceInvite, true, domain.OrganizationKindPersonal, true, nil},
	}

	for _, tt := range tests {
		got, err := SignupOrganizationPlanFor(tt.mode, tt.source)
		if !errors.Is(err, tt.wantErr) {
			t.Fatalf("mode=%s source=%s expected err %v, got %v", tt.mode, tt.source, tt.wantErr, err)
		}
		if got.CreateInitialOrg != tt.createInitialOrg ||
			got.InitialOrgKind != tt.initialOrgKind ||
			got.AcceptInvitation != tt.acceptInvitation {
			t.Fatalf("mode=%s source=%s got %+v", tt.mode, tt.source, got)
		}
	}
}

func TestOrgModeCapabilities(t *testing.T) {
	tests := []struct {
		mode                    OrgMode
		allowsInvitations       bool
		allowsOrgSwitching      bool
		allowsUserCreatedTeams  bool
		allowsLeaveOrg          bool
		hasVisibleOrganizations bool
	}{
		{OrgModePersonal, false, false, false, false, false},
		{OrgModeSingle, true, false, false, false, true},
		{OrgModeMulti, true, true, true, true, true},
	}

	for _, tt := range tests {
		if tt.mode.AllowsInvitations() != tt.allowsInvitations ||
			tt.mode.AllowsOrgSwitching() != tt.allowsOrgSwitching ||
			tt.mode.AllowsUserCreatedTeamOrgs() != tt.allowsUserCreatedTeams ||
			tt.mode.AllowsLeaveOrg() != tt.allowsLeaveOrg ||
			tt.mode.HasVisibleOrganizations() != tt.hasVisibleOrganizations {
			t.Fatalf("mode=%s capabilities mismatch", tt.mode)
		}
	}
}
