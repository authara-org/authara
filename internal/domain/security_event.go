package domain

import (
	"time"

	"github.com/google/uuid"
)

type SecurityEventType string

const (
	SecurityEventAuthenticationLogin           SecurityEventType = "authentication.login"
	SecurityEventAuthenticationReauthenticated SecurityEventType = "authentication.reauthenticated"
	SecurityEventSessionRefresh                SecurityEventType = "session.refresh"
	SecurityEventSessionRefreshTokenReuse      SecurityEventType = "session.refresh_token_reuse"
	SecurityEventSessionLogout                 SecurityEventType = "session.logout"
	SecurityEventSessionRevoked                SecurityEventType = "session.revoked"
	SecurityEventCredentialPasswordAdded       SecurityEventType = "credential.password_added"
	SecurityEventCredentialPasswordChanged     SecurityEventType = "credential.password_changed"
	SecurityEventCredentialPasswordReset       SecurityEventType = "credential.password_reset"
	SecurityEventCredentialProviderLinked      SecurityEventType = "credential.provider_linked"
	SecurityEventCredentialProviderChanged     SecurityEventType = "credential.provider_changed"
	SecurityEventCredentialProviderRemoved     SecurityEventType = "credential.provider_removed"
	SecurityEventCredentialPasskeyAdded        SecurityEventType = "credential.passkey_added"
	SecurityEventCredentialPasskeyRemoved      SecurityEventType = "credential.passkey_removed"
	SecurityEventAccountEmailChanged           SecurityEventType = "account.email_changed"
	SecurityEventPasskeyCloneWarning           SecurityEventType = "passkey.clone_warning"
)

var allSecurityEventTypes = []SecurityEventType{
	SecurityEventAuthenticationLogin,
	SecurityEventAuthenticationReauthenticated,
	SecurityEventSessionRefresh,
	SecurityEventSessionRefreshTokenReuse,
	SecurityEventSessionLogout,
	SecurityEventSessionRevoked,
	SecurityEventCredentialPasswordAdded,
	SecurityEventCredentialPasswordChanged,
	SecurityEventCredentialPasswordReset,
	SecurityEventCredentialProviderLinked,
	SecurityEventCredentialProviderChanged,
	SecurityEventCredentialProviderRemoved,
	SecurityEventCredentialPasskeyAdded,
	SecurityEventCredentialPasskeyRemoved,
	SecurityEventAccountEmailChanged,
	SecurityEventPasskeyCloneWarning,
}

func SecurityEventTypes() []SecurityEventType {
	return append([]SecurityEventType(nil), allSecurityEventTypes...)
}

func StandardSecurityEventTypes() []SecurityEventType {
	out := make([]SecurityEventType, 0, len(allSecurityEventTypes)-2)
	for _, eventType := range allSecurityEventTypes {
		if eventType != SecurityEventSessionRefresh && eventType != SecurityEventSessionLogout {
			out = append(out, eventType)
		}
	}
	return out
}

func IsSecurityEventType(value string) bool {
	for _, eventType := range allSecurityEventTypes {
		if value == string(eventType) {
			return true
		}
	}
	return false
}

type SecurityEventOutcome string

const (
	SecurityEventOutcomeSuccess SecurityEventOutcome = "success"
	SecurityEventOutcomeDenied  SecurityEventOutcome = "denied"
	SecurityEventOutcomeFailure SecurityEventOutcome = "failure"
)

type SecurityEventActorType string

const (
	SecurityEventActorAnonymous SecurityEventActorType = "anonymous"
	SecurityEventActorUser      SecurityEventActorType = "user"
	SecurityEventActorSystem    SecurityEventActorType = "system"
)

const (
	SecurityEventReasonInvalidCredentials  = "invalid_credentials"
	SecurityEventReasonInvalidAssertion    = "invalid_assertion"
	SecurityEventReasonAccessPolicy        = "access_policy"
	SecurityEventReasonAccountLinkRequired = "account_link_required"
	SecurityEventReasonProviderDisabled    = "provider_disabled"
	SecurityEventReasonAudienceForbidden   = "audience_forbidden"
	SecurityEventReasonUserDisabled        = "user_disabled"
	SecurityEventReasonMethodUnavailable   = "authentication_method_unavailable"
	SecurityEventReasonRefreshTokenReuse   = "refresh_token_reuse"
	SecurityEventReasonUserRequested       = "user_requested"
	SecurityEventReasonSecurityContainment = "security_containment"
)

// SecurityEvent contains only allowlisted, non-secret security context. Raw
// identifiers, tokens, credentials, email addresses, IP addresses and user
// agents must never be added to this record.
type SecurityEvent struct {
	ID                   uuid.UUID              `json:"id"`
	CreatedAt            time.Time              `json:"created_at"`
	Type                 SecurityEventType      `json:"type"`
	Outcome              SecurityEventOutcome   `json:"outcome"`
	ReasonCode           string                 `json:"reason_code,omitempty"`
	ActorType            SecurityEventActorType `json:"actor_type"`
	ActorUserID          *uuid.UUID             `json:"actor_user_id,omitempty"`
	UserID               *uuid.UUID             `json:"subject_user_id,omitempty"`
	SessionID            *uuid.UUID             `json:"session_id,omitempty"`
	OrganizationID       *uuid.UUID             `json:"organization_id,omitempty"`
	PasskeyID            *uuid.UUID             `json:"passkey_id,omitempty"`
	AuthenticationMethod AuthenticationMethod   `json:"authentication_method,omitempty"`
	Response             string                 `json:"response,omitempty"`
}
