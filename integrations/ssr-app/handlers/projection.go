package handlers

import (
	"encoding/json"
	"sync"

	"github.com/authara-org/authara-go/authara"
)

type orgState struct {
	ID          string
	Name        string
	Members     map[string]memberState
	Invitations map[string]invitationState
}

type memberState struct {
	ID    string
	Email string
	Role  string
}

type invitationState struct {
	ID        string
	Email     string
	Role      string
	Status    string
	ExpiresAt string
	InviteURL string
}

var projection = struct {
	sync.Mutex
	Orgs map[string]*orgState
}{
	// ponytail: in-memory demo projection; use a DB if the SSR app needs restart-safe state.
	Orgs: map[string]*orgState{},
}

func RecordWebhook(evt *authara.WebhookEvent) error {
	switch evt.Type {
	case "organization.deleted":
		var data struct {
			OrganizationID string `json:"organization_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		deleteOrganization(data.OrganizationID)

	case "organization.created", "organization.updated":
		var data struct {
			OrganizationID string `json:"organization_id"`
			Name           string `json:"name"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordOrganization(internalOrganizationDTO{ID: data.OrganizationID, Name: data.Name}, nil)

	case "organization.invitation.created":
		var data invitationState
		var raw struct {
			ID             string `json:"invitation_id"`
			OrganizationID string `json:"organization_id"`
			Email          string `json:"email"`
			Role           string `json:"role"`
			ExpiresAt      string `json:"expires_at"`
		}
		if err := json.Unmarshal(evt.Data, &raw); err != nil {
			return err
		}
		data.ID = raw.ID
		data.Email = raw.Email
		data.Role = raw.Role
		data.Status = "pending"
		data.ExpiresAt = raw.ExpiresAt
		recordInvitationForOrg(raw.OrganizationID, data)

	case "organization.invitation.accepted":
		var data struct {
			InvitationID     string `json:"invitation_id"`
			OrganizationID   string `json:"organization_id"`
			Email            string `json:"email"`
			Role             string `json:"role"`
			AcceptedByUserID string `json:"accepted_by_user_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordInvitationAccepted(data.OrganizationID, data.InvitationID, data.AcceptedByUserID, data.Email, data.Role)

	case "organization.invitation.revoked":
		var data struct {
			InvitationID   string `json:"invitation_id"`
			OrganizationID string `json:"organization_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordInvitationRevoked(data.OrganizationID, data.InvitationID)

	case "organization.membership.created", "organization.membership.updated":
		var data struct {
			OrganizationID string `json:"organization_id"`
			UserID         string `json:"user_id"`
			Role           string `json:"role"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		recordMember(data.OrganizationID, memberState{ID: data.UserID, Role: data.Role})

	case "organization.membership.deleted":
		var data struct {
			OrganizationID string `json:"organization_id"`
			UserID         string `json:"user_id"`
		}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return err
		}
		deleteMember(data.OrganizationID, data.UserID)
	}
	return nil
}

func upsertCurrentUser(user *currentUser) {
	if user.Organization == nil {
		return
	}
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(user.Organization.ID, user.Organization.Name)
	org.Members[user.ID] = memberState{
		ID:    user.ID,
		Email: user.Email,
		Role:  user.Organization.Role,
	}
}

func recordInvitation(inv invitationDTO) {
	recordInvitationForOrg(inv.OrganizationID, invitationState{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      inv.Role,
		Status:    inv.Status,
		ExpiresAt: inv.ExpiresAt,
		InviteURL: inv.InviteURL,
	})
}

func recordInternalInvitation(inv internalInvitationDTO) {
	recordInvitationForOrg(inv.OrganizationID, invitationState{
		ID:        inv.ID,
		Email:     inv.Email,
		Role:      inv.Role,
		Status:    inv.Status,
		ExpiresAt: inv.ExpiresAt,
		InviteURL: inv.InviteURL,
	})
}

func recordOrganization(org internalOrganizationDTO, member *internalMembershipDTO) {
	projection.Lock()
	defer projection.Unlock()

	state := ensureOrgLocked(org.ID, org.Name)
	if member != nil && member.UserID != "" {
		state.Members[member.UserID] = memberState{ID: member.UserID, Role: member.Role}
	}
}

func deleteOrganization(orgID string) {
	projection.Lock()
	defer projection.Unlock()

	delete(projection.Orgs, orgID)
}

func recordInvitationForOrg(orgID string, inv invitationState) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	if inv.Status == "" {
		inv.Status = "pending"
	}
	org.Invitations[inv.ID] = inv
}

func recordInvitationAccepted(orgID, invitationID, userID, email, role string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	inv := org.Invitations[invitationID]
	inv.Status = "accepted"
	org.Invitations[invitationID] = inv
	org.Members[userID] = memberState{ID: userID, Email: email, Role: role}
}

func recordInvitationRevoked(orgID, invitationID string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	inv := org.Invitations[invitationID]
	inv.Status = "revoked"
	org.Invitations[invitationID] = inv
}

func recordMember(orgID string, member memberState) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	current := org.Members[member.ID]
	if member.Email == "" {
		member.Email = current.Email
	}
	org.Members[member.ID] = member
}

func deleteMember(orgID, userID string) {
	projection.Lock()
	defer projection.Unlock()

	org := ensureOrgLocked(orgID, "")
	delete(org.Members, userID)
}

func ensureOrgLocked(id, name string) *orgState {
	org := projection.Orgs[id]
	if org == nil {
		org = &orgState{
			ID:          id,
			Members:     map[string]memberState{},
			Invitations: map[string]invitationState{},
		}
		projection.Orgs[id] = org
	}
	if name != "" {
		org.Name = name
	}
	if org.Name == "" {
		org.Name = id
	}
	return org
}
