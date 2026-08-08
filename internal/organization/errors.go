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
	ErrOrganizationActorNotAllowed           = errors.New("organization actor is not allowed")
	ErrOrganizationInviteEmailMismatch       = errors.New("organization invitation email mismatch")
	ErrOrganizationSingleMembershipConflict  = errors.New("single organization membership conflict")
	ErrLastOrganizationOwner                 = errors.New("operation would leave organization without an owner")
	ErrLastOrganizationMember                = errors.New("last organization member must delete the organization")
	ErrOrganizationHasOtherMembers           = errors.New("organization has other members")
	ErrPersonalOrganizationImmutable         = errors.New("personal organization creator cannot leave and personal organization cannot be deleted")
	ErrInvalidOrganizationOwnershipTransfer  = errors.New("new organization owner must differ from current owner")
	ErrInvalidOrganizationMode               = errors.New("invalid organization mode")
	ErrInvalidOrganizationRole               = errors.New("invalid organization role")
	ErrOrganizationOperationForbidden        = errors.New("organization operation forbidden")
	ErrInvalidOrganizationInvitationEmail    = errors.New("invalid organization invitation email")
	ErrInvalidOrganizationInvitationMetadata = errors.New("invalid organization invitation metadata")
	ErrInvalidOrganizationInvitationToken    = errors.New("invalid organization invitation token")
)
