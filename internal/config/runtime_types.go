package config

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	configruntime "github.com/authara-org/authara/internal/config/runtime"
	"github.com/google/uuid"
)

type Key = configruntime.Key

const (
	KeyUIDefaultReturnTo Key = "ui.default_return_to"

	KeyAuthenticationUsernameLoginEnabled Key = "authentication.username_login_enabled"

	KeyTokenAccessTTL Key = "token.access_ttl"

	KeySessionTTL             Key = "session.ttl"
	KeySessionRefreshTokenTTL Key = "session.refresh_token_ttl"
	KeySessionRotation        Key = "session.refresh_token_rotation"

	KeyOrganizationPublicManagementEnabled Key = "organization.public_management_enabled"
	KeyOrganizationInvitationTTL           Key = "organization.invitation_ttl"

	KeyAccessPolicyAllowlistEnabled Key = "access_policy.allowlist_enabled"

	KeyAdminAuditRetention Key = "admin.audit_retention"

	KeyEmailJobMaxAttempts     Key = "email.job_max_attempts"
	KeyEmailCleanupSentAfter   Key = "email.cleanup_sent_after"
	KeyEmailCleanupFailedAfter Key = "email.cleanup_failed_after"

	KeyWebhookEnabledEvents        Key = "webhook.enabled_events"
	KeyWebhookTimeout              Key = "webhook.timeout"
	KeyWebhookMaxDeliveryAttempts  Key = "webhook.max_delivery_attempts"
	KeyWebhookProcessingStaleAfter Key = "webhook.processing_stale_after"
	KeyWebhookDeliveredRetention   Key = "webhook.delivered_retention"
	KeyWebhookFailedRetention      Key = "webhook.failed_retention"
	KeyWebhookMaintenanceBatchSize Key = "webhook.maintenance_batch_size"

	KeyChallengeEnabled               Key = "challenge.enabled"
	KeyChallengeTTL                   Key = "challenge.ttl"
	KeyChallengeVerificationCodeTTL   Key = "challenge.verification_code_ttl"
	KeyChallengeMaxAttempts           Key = "challenge.max_attempts"
	KeyChallengeMaxResends            Key = "challenge.max_resends"
	KeyChallengeMinimumResendInterval Key = "challenge.minimum_resend_interval"

	KeyRateLimitLoginIPLimit             Key = "rate_limit.login.ip.limit"
	KeyRateLimitLoginIPWindow            Key = "rate_limit.login.ip.window"
	KeyRateLimitLoginEmailLimit          Key = "rate_limit.login.email.limit"
	KeyRateLimitLoginEmailWindow         Key = "rate_limit.login.email.window"
	KeyRateLimitSignupIPLimit            Key = "rate_limit.signup.ip.limit"
	KeyRateLimitSignupIPWindow           Key = "rate_limit.signup.ip.window"
	KeyRateLimitSignupEmailLimit         Key = "rate_limit.signup.email.limit"
	KeyRateLimitSignupEmailWindow        Key = "rate_limit.signup.email.window"
	KeyRateLimitPasswordResetIPLimit     Key = "rate_limit.password_reset.ip.limit"
	KeyRateLimitPasswordResetIPWindow    Key = "rate_limit.password_reset.ip.window"
	KeyRateLimitPasswordResetEmailLimit  Key = "rate_limit.password_reset.email.limit"
	KeyRateLimitPasswordResetEmailWindow Key = "rate_limit.password_reset.email.window"
	KeyRateLimitPasskeyLoginIPLimit      Key = "rate_limit.passkey_login.ip.limit"
	KeyRateLimitPasskeyLoginIPWindow     Key = "rate_limit.passkey_login.ip.window"
	KeyRateLimitChallengeVerifyIPLimit   Key = "rate_limit.challenge_verify.ip.limit"
	KeyRateLimitChallengeVerifyIPWindow  Key = "rate_limit.challenge_verify.ip.window"
	KeyRateLimitChallengeResendIPLimit   Key = "rate_limit.challenge_resend.ip.limit"
	KeyRateLimitChallengeResendIPWindow  Key = "rate_limit.challenge_resend.ip.window"
	KeyRateLimitCleanupEvery             Key = "rate_limit.cleanup_every"
	KeyRateLimitMaxEntries               Key = "rate_limit.max_entries"
)

type Control string

const (
	ControlEnvironment Control = "environment"
	ControlOperator    Control = "operator"
	ControlHybrid      Control = "hybrid"
)

type Reload string

const (
	ReloadStartup Reload = "startup"
	ReloadDynamic Reload = "dynamic"
)

type ValueType string

const (
	TypeBool     ValueType = "bool"
	TypeInt      ValueType = "int"
	TypeDuration ValueType = "duration"
	TypeString   ValueType = "string"
	TypeEnum     ValueType = "enum"
	TypeURL      ValueType = "url"
	TypeCSV      ValueType = "csv"
	TypeMap      ValueType = "map"
)

type Source string

const (
	SourceEnvironment Source = "environment"
	SourceOperator    Source = "operator"
	SourceDefault     Source = "default"
	SourceUnset       Source = "unset"
)

var (
	ErrUnknownSetting   = errors.New("unknown runtime setting")
	ErrSettingLocked    = errors.New("runtime setting is locked")
	ErrRevisionConflict = configruntime.ErrRevisionConflict
	ErrInvalidValue     = errors.New("invalid runtime setting value")
	ErrMissingActor     = errors.New("runtime setting actor is missing")
)

// ChallengePolicy is published as one immutable value so an operation cannot
// observe a mixture of revisions while reading related settings.
type ChallengePolicy struct {
	Enabled               bool
	TTL                   time.Duration
	VerificationCodeTTL   time.Duration
	MaxAttempts           int
	MaxResends            int
	MinimumResendInterval time.Duration
}

type ChallengePolicyReader interface {
	CurrentChallenge() ChallengePolicy
}

type StaticChallengePolicy struct {
	Policy ChallengePolicy
}

func (s StaticChallengePolicy) CurrentChallenge() ChallengePolicy { return s.Policy }

type UIPolicy struct {
	DefaultReturnTo string
}

type UIPolicyReader interface {
	CurrentUI() UIPolicy
}

type UIPolicyReaderFunc func() UIPolicy

func (f UIPolicyReaderFunc) CurrentUI() UIPolicy { return f() }

type AuthenticationPolicy struct {
	UsernameLoginEnabled bool
}

type AuthenticationPolicyReader interface {
	CurrentAuthentication() AuthenticationPolicy
}

type AuthenticationPolicyReaderFunc func() AuthenticationPolicy

func (f AuthenticationPolicyReaderFunc) CurrentAuthentication() AuthenticationPolicy { return f() }

type TokenPolicy struct {
	AccessTokenTTL time.Duration
}

type TokenPolicyReader interface {
	CurrentToken() TokenPolicy
}

type TokenPolicyReaderFunc func() TokenPolicy

func (f TokenPolicyReaderFunc) CurrentToken() TokenPolicy { return f() }

type SessionPolicy struct {
	SessionTTL           time.Duration
	RefreshTokenTTL      time.Duration
	RefreshTokenRotation time.Duration
}

type SessionPolicyReader interface {
	CurrentSession() SessionPolicy
}

type SessionPolicyReaderFunc func() SessionPolicy

func (f SessionPolicyReaderFunc) CurrentSession() SessionPolicy { return f() }

// SessionCookiePolicy is an HTTP-consumer snapshot combining the two policy
// groups needed to write a consistent access/refresh cookie pair.
type SessionCookiePolicy struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type SessionCookiePolicyReader interface {
	CurrentSessionCookies() SessionCookiePolicy
}

type SessionCookiePolicyReaderFunc func() SessionCookiePolicy

func (f SessionCookiePolicyReaderFunc) CurrentSessionCookies() SessionCookiePolicy { return f() }

type OrganizationPolicy struct {
	PublicManagementEnabled bool
	InvitationTTL           time.Duration
}

type OrganizationPolicyReader interface {
	CurrentOrganization() OrganizationPolicy
}

type OrganizationPolicyReaderFunc func() OrganizationPolicy

func (f OrganizationPolicyReaderFunc) CurrentOrganization() OrganizationPolicy { return f() }

type AllowlistPolicy struct {
	AllowlistEnabled bool
}

type AllowlistPolicyReader interface {
	CurrentAllowlist() AllowlistPolicy
}

type AllowlistPolicyReaderFunc func() AllowlistPolicy

func (f AllowlistPolicyReaderFunc) CurrentAllowlist() AllowlistPolicy { return f() }

type AdminPolicy struct {
	AuditRetention time.Duration
}

type AdminPolicyReader interface {
	CurrentAdmin() AdminPolicy
}

type AdminPolicyReaderFunc func() AdminPolicy

func (f AdminPolicyReaderFunc) CurrentAdmin() AdminPolicy { return f() }

type EmailPolicy struct {
	JobMaxAttempts     int
	CleanupSentAfter   time.Duration
	CleanupFailedAfter time.Duration
}

type EmailPolicyReader interface {
	CurrentEmail() EmailPolicy
}

type EmailPolicyReaderFunc func() EmailPolicy

func (f EmailPolicyReaderFunc) CurrentEmail() EmailPolicy { return f() }

type WebhookPolicy struct {
	EnabledEvents        []string
	Timeout              time.Duration
	MaxDeliveryAttempts  int
	ProcessingStaleAfter time.Duration
	DeliveredRetention   time.Duration
	FailedRetention      time.Duration
	MaintenanceBatchSize int
}

type WebhookPolicyReader interface {
	CurrentWebhook() WebhookPolicy
}

type WebhookPolicyReaderFunc func() WebhookPolicy

func (f WebhookPolicyReaderFunc) CurrentWebhook() WebhookPolicy { return f() }

// RateLimitPolicy is published atomically so one limiter call observes a
// consistent set of thresholds, windows, and in-memory safety controls.
type RateLimitPolicy struct {
	LoginIPLimit     int
	LoginIPWindow    time.Duration
	LoginEmailLimit  int
	LoginEmailWindow time.Duration

	SignupIPLimit     int
	SignupIPWindow    time.Duration
	SignupEmailLimit  int
	SignupEmailWindow time.Duration

	PasswordResetIPLimit     int
	PasswordResetIPWindow    time.Duration
	PasswordResetEmailLimit  int
	PasswordResetEmailWindow time.Duration

	PasskeyLoginIPLimit  int
	PasskeyLoginIPWindow time.Duration

	ChallengeVerifyIPLimit  int
	ChallengeVerifyIPWindow time.Duration

	ChallengeResendIPLimit  int
	ChallengeResendIPWindow time.Duration

	CleanupEvery int
	MaxEntries   int
}

type Definition struct {
	Key          Key
	Name         string
	Description  string
	Environment  string
	Control      Control
	Reload       Reload
	Sensitive    bool
	Group        string
	Type         ValueType
	DefaultValue string
	Required     bool
	HasDefault   bool
	Minimum      string
	Maximum      string
	Allowed      []string
	Impact       string

	defaultValue any
	minInt       *int
	maxInt       *int
	minDuration  *time.Duration
	maxDuration  *time.Duration
}

type Description struct {
	Definition
	EffectiveValue    string
	EffectiveSource   Source
	Locked            bool
	PersistedOverride *string
	Revision          int64
	DormantOverride   bool
}

type PersistedOverride = configruntime.PersistedOverride

type PersistedState = configruntime.PersistedState

type Mutation = configruntime.Mutation

type RuntimeSettingsStore interface {
	LoadRuntimeSettings(context.Context) (PersistedState, error)
	UpsertRuntimeSettingOverride(context.Context, Mutation) (PersistedOverride, error)
	DeleteRuntimeSettingOverride(context.Context, Key, int64, int64, uuid.UUID, json.RawMessage) (int64, error)
}
