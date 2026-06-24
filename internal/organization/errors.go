package organization

import "errors"

var (
	ErrOrganizationInvitationExpired         = errors.New("organization invitation expired")
	ErrOrganizationInvitationAlreadyAccepted = errors.New("organization invitation already accepted")
	ErrOrganizationInvitationRevoked         = errors.New("organization invitation revoked")
	ErrOrganizationInvitationAlreadyPending  = errors.New("organization invitation already pending")
	ErrOrganizationMemberAlreadyExists       = errors.New("organization member already exists")
	ErrOrganizationInviteForbidden           = errors.New("organization invite forbidden")
	ErrOrganizationActorNotMember            = errors.New("organization invitation actor is not a member")
	ErrOrganizationInviteEmailMismatch       = errors.New("organization invitation email mismatch")
	ErrOrganizationSingleMembershipConflict  = errors.New("single organization membership conflict")
	ErrInvalidOrganizationMode               = errors.New("invalid organization mode")
	ErrInvalidOrganizationInvitationEmail    = errors.New("invalid organization invitation email")
	ErrInvalidOrganizationInvitationToken    = errors.New("invalid organization invitation token")
)
