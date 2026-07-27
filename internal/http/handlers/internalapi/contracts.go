package internalapi

import (
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

const (
	codeUserNotFound              response.ErrorCode = "user_not_found"
	codeOrganizationNotFound      response.ErrorCode = "organization_not_found"
	codeMembershipNotFound        response.ErrorCode = "membership_not_found"
	codeInvitationNotFound        response.ErrorCode = "invitation_not_found"
	codeActorNotMember            response.ErrorCode = "actor_not_member"
	codeActorNotAllowed           response.ErrorCode = "actor_not_allowed"
	codeAlreadyMember             response.ErrorCode = "already_member"
	codeInvitationAlreadyPending  response.ErrorCode = "invitation_already_pending"
	codeInvitationAlreadyAccepted response.ErrorCode = "invitation_already_accepted"
	codeInvitationRevoked         response.ErrorCode = "invitation_revoked"
	codeInvitationExpired         response.ErrorCode = "invitation_expired"
)

var (
	GetPublicCapabilitiesErrors                = contract.MustOperationErrors("getPublicCapabilities")
	CreateInternalOrganizationErrors           = contract.MustOperationErrors("createInternalOrganization")
	GetPublicOrganizationErrors                = contract.MustOperationErrors("getPublicOrganization")
	UpdatePublicOrganizationErrors             = contract.MustOperationErrors("updatePublicOrganization")
	ListPublicOrganizationMembersErrors        = contract.MustOperationErrors("listPublicOrganizationMembers")
	GetPublicOrganizationMemberErrors          = contract.MustOperationErrors("getPublicOrganizationMember")
	ListPublicOrganizationInvitationsErrors    = contract.MustOperationErrors("listPublicOrganizationInvitations")
	CreateInternalOrganizationInvitationErrors = contract.MustOperationErrors("createInternalOrganizationInvitation")
	GetPublicOrganizationInvitationErrors      = contract.MustOperationErrors("getPublicOrganizationInvitation")
	RevokePublicOrganizationInvitationErrors   = contract.MustOperationErrors("revokePublicOrganizationInvitation")
	ResendInternalOrganizationInvitationErrors = contract.MustOperationErrors("resendInternalOrganizationInvitation")
	ListPublicUserMembershipsErrors            = contract.MustOperationErrors("listPublicUserMemberships")
)
