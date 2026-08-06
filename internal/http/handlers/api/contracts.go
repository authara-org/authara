package api

import (
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
)

const (
	codeAccountLinkRequired        response.ErrorCode = "account_link_required"
	codePasskeyAlreadyExists       response.ErrorCode = "passkey_already_exists"
	codePasskeyRegistrationInvalid response.ErrorCode = "passkey_registration_invalid"
)

var (
	GetCsrfTokenErrors                   = contract.MustOperationErrors("getCsrfToken")
	GetGoogleLoginOptionsErrors          = contract.MustOperationErrors("getGoogleLoginOptions")
	LoginWithGoogleErrors                = contract.MustOperationErrors("loginWithGoogle")
	LoginWithPasswordErrors              = contract.MustOperationErrors("loginWithPassword")
	SignupDirectErrors                   = contract.MustOperationErrors("signupDirect")
	StartSignupChallengeErrors           = contract.MustOperationErrors("startSignupChallenge")
	VerifySignupChallengeErrors          = contract.MustOperationErrors("verifySignupChallenge")
	StartPasswordResetChallengeErrors    = contract.MustOperationErrors("startPasswordResetChallenge")
	VerifyPasswordResetChallengeErrors   = contract.MustOperationErrors("verifyPasswordResetChallenge")
	ResendChallengeErrors                = contract.MustOperationErrors("resendChallenge")
	BeginPasskeyAuthenticationErrors     = contract.MustOperationErrors("beginPasskeyAuthentication")
	FinishPasskeyAuthenticationErrors    = contract.MustOperationErrors("finishPasskeyAuthentication")
	BeginPasskeyRegistrationErrors       = contract.MustOperationErrors("beginPasskeyRegistration")
	FinishPasskeyRegistrationErrors      = contract.MustOperationErrors("finishPasskeyRegistration")
	LogoutErrors                         = contract.MustOperationErrors("logout")
	RefreshSessionErrors                 = contract.MustOperationErrors("refreshSession")
	RefreshTokensErrors                  = contract.MustOperationErrors("refreshTokens")
	GetCurrentUserErrors                 = contract.MustOperationErrors("getCurrentUser")
	SetCurrentUserPasswordErrors         = contract.MustOperationErrors("setCurrentUserPassword")
	ListCurrentUserOrganizationsErrors   = contract.MustOperationErrors("listCurrentUserOrganizations")
	GetCurrentOrganizationErrors         = contract.MustOperationErrors("getCurrentOrganization")
	ListCurrentOrganizationMembersErrors = contract.MustOperationErrors("listCurrentOrganizationMembers")
	SwitchOrganizationErrors             = contract.MustOperationErrors("switchOrganization")
)
