package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound                     = errors.New("user not found")
	ErrSessionNotFound                  = errors.New("session not found")
	ErrorAuthProviderNotFound           = errors.New("auth_provider not found")
	ErrRefreshTokenNotFound             = errors.New("refresh_token not found")
	ErrorChallengeNotFound              = errors.New("challenge not found")
	ErrorChallengeAlreadyConsumed       = errors.New("challenge already consumed")
	ErrorVerificationCodeNotFound       = errors.New("verification code not found")
	ErrorEmailJobNotFound               = errors.New("email job not found")
	ErrorPendingSignupActionNotFound    = errors.New("pending signup action not found")
	ErrorPendingPasswordResetNotFound   = errors.New("pending password reset not found")
	ErrorPendingEmailChangeNotFound     = errors.New("pending email change not found")
	ErrorRoleNotFound                   = errors.New("role not found")
	ErrorPendingProviderLinkNotFound    = errors.New("pending provider link not found")
	ErrPasskeyNotFound                  = errors.New("passkey not found")
	ErrPasskeyAlreadyExists             = errors.New("passkey already exists")
	ErrWebAuthnChallengeNotFound        = errors.New("webauthn challenge not found")
	ErrWebAuthnChallengeAlreadyConsumed = errors.New("webauthn challenge already consumed")
	ErrAllowedEmailNotFound             = errors.New("allowed email not found")
	ErrAllowedEmailAlreadyExists        = errors.New("allowed email already exists")
	ErrOrganizationNotFound             = errors.New("organization not found")
	ErrOrganizationMembershipNotFound   = errors.New("organization membership not found")
	ErrOrganizationInvitationNotFound   = errors.New("organization invitation not found")
	ErrInvalidOrganizationName          = errors.New("invalid organization name")
)

const (
	ConstraintUserEmail           = "unique_user_email"
	ConstraintUserUsername        = "unique_user_username"
	ConstraintPasskeyCredentialID = "unique_passkey_credential_id"
	ConstraintAllowedEmailEmail   = "unique_allowed_email"
	ConstraintPersonalOrgUser     = "unique_personal_org_created_by_user"
	ConstraintInvitationTokenHash = "organization_invitations_token_hash_key"
	ConstraintActiveInvitation    = "unique_active_organization_invitation_email"

	uniqueViolationCode = "23505"
)

func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	if pgErr.Code != uniqueViolationCode {
		return false
	}

	if constraint == "" {
		return true
	}

	return pgErr.ConstraintName == constraint
}
