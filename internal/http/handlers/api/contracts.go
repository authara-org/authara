package api

import (
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

const (
	codeAccountLinkRequired            response.ErrorCode = "account_link_required"
	codeAuthMethodAlreadyLinked        response.ErrorCode = "auth_method_already_linked"
	codeCannotRemoveLastAuthMethod     response.ErrorCode = "cannot_remove_last_auth_method"
	codePasskeyAlreadyExists           response.ErrorCode = "passkey_already_exists"
	codePasskeyRegistrationInvalid     response.ErrorCode = "passkey_registration_invalid"
	codePasswordAlreadyExists          response.ErrorCode = "password_already_exists"
	codeUsernameTaken                  response.ErrorCode = "username_taken"
	codeInvitationNotFound             response.ErrorCode = "invitation_not_found"
	codeInvitationEmailMismatch        response.ErrorCode = "invitation_email_mismatch"
	codeInvitationAlreadyAccepted      response.ErrorCode = "invitation_already_accepted"
	codeInvitationRevoked              response.ErrorCode = "invitation_revoked"
	codeInvitationExpired              response.ErrorCode = "invitation_expired"
	codeOrganizationMembershipConflict response.ErrorCode = "organization_membership_conflict"
	codeInvitationFlowMismatch         response.ErrorCode = "invitation_flow_mismatch"
	codeProviderLinkExpired            response.ErrorCode = "provider_link_expired"
	codePasswordProviderNotFound       response.ErrorCode = "password_provider_not_found"
)

var (
	GetCsrfTokenErrors                              = contract.MustOperationErrors("getCsrfToken")
	GetGoogleLoginOptionsErrors                     = contract.MustOperationErrors("getGoogleLoginOptions")
	LoginWithGoogleErrors                           = contract.MustOperationErrors("loginWithGoogle")
	LoginWithPasswordErrors                         = contract.MustOperationErrors("loginWithPassword")
	SignupDirectErrors                              = contract.MustOperationErrors("signupDirect")
	StartSignupChallengeErrors                      = contract.MustOperationErrors("startSignupChallenge")
	VerifySignupChallengeErrors                     = contract.MustOperationErrors("verifySignupChallenge")
	StartPasswordResetChallengeErrors               = contract.MustOperationErrors("startPasswordResetChallenge")
	VerifyPasswordResetChallengeErrors              = contract.MustOperationErrors("verifyPasswordResetChallenge")
	ResendChallengeErrors                           = contract.MustOperationErrors("resendChallenge")
	BeginPasskeyAuthenticationErrors                = contract.MustOperationErrors("beginPasskeyAuthentication")
	FinishPasskeyAuthenticationErrors               = contract.MustOperationErrors("finishPasskeyAuthentication")
	BeginPasskeyRegistrationErrors                  = contract.MustOperationErrors("beginPasskeyRegistration")
	FinishPasskeyRegistrationErrors                 = contract.MustOperationErrors("finishPasskeyRegistration")
	LogoutErrors                                    = contract.MustOperationErrors("logout")
	RefreshSessionErrors                            = contract.MustOperationErrors("refreshSession")
	RefreshTokensErrors                             = contract.MustOperationErrors("refreshTokens")
	GetCurrentUserErrors                            = contract.MustOperationErrors("getCurrentUser")
	GetCurrentAccountErrors                         = contract.MustOperationErrors("getCurrentAccount")
	ChangeCurrentUsernameErrors                     = contract.MustOperationErrors("changeCurrentUsername")
	StartCurrentUserEmailChangeErrors               = contract.MustOperationErrors("startCurrentUserEmailChange")
	VerifyCurrentUserEmailChangeErrors              = contract.MustOperationErrors("verifyCurrentUserEmailChange")
	AddCurrentUserPasswordErrors                    = contract.MustOperationErrors("addCurrentUserPassword")
	ChangeCurrentUserPasswordErrors                 = contract.MustOperationErrors("changeCurrentUserPassword")
	LinkCurrentUserGoogleErrors                     = contract.MustOperationErrors("linkCurrentUserGoogle")
	UnlinkCurrentUserAuthMethodErrors               = contract.MustOperationErrors("unlinkCurrentUserAuthMethod")
	DeleteCurrentUserPasskeyErrors                  = contract.MustOperationErrors("deleteCurrentUserPasskey")
	RevokeCurrentUserSessionErrors                  = contract.MustOperationErrors("revokeCurrentUserSession")
	RevokeCurrentUserOtherSessionsErrors            = contract.MustOperationErrors("revokeCurrentUserOtherSessions")
	SetCurrentUserPasswordErrors                    = contract.MustOperationErrors("setCurrentUserPassword")
	ListCurrentUserOrganizationsErrors              = contract.MustOperationErrors("listCurrentUserOrganizations")
	GetCurrentOrganizationErrors                    = contract.MustOperationErrors("getCurrentOrganization")
	ListCurrentOrganizationMembersErrors            = contract.MustOperationErrors("listCurrentOrganizationMembers")
	SwitchOrganizationErrors                        = contract.MustOperationErrors("switchOrganization")
	PreviewInvitationErrors                         = contract.MustOperationErrors("previewInvitation")
	AcceptInvitationErrors                          = contract.MustOperationErrors("acceptInvitation")
	LoginAndAcceptInvitationErrors                  = contract.MustOperationErrors("loginAndAcceptInvitation")
	AuthenticateAndAcceptInvitationWithGoogleErrors = contract.MustOperationErrors("authenticateAndAcceptInvitationWithGoogle")
	StartGoogleAccountRecoveryLinkErrors            = contract.MustOperationErrors("startGoogleAccountRecoveryLink")
	CompleteAccountRecoveryLinkWithPasswordErrors   = contract.MustOperationErrors("completeAccountRecoveryLinkWithPassword")
	CompleteAccountRecoveryLinkWithGoogleErrors     = contract.MustOperationErrors("completeAccountRecoveryLinkWithGoogle")
)
