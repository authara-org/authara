package config

import (
	"strconv"
	"time"
)

func intPointer(value int) *int { return &value }

func durationPointer(value time.Duration) *time.Duration { return &value }

var runtimeDefinitions = func() []Definition {
	definitions := generalPolicyDefinitions()
	definitions = append(definitions, []Definition{
		{
			Key: KeyChallengeEnabled, Name: "Challenge flows enabled",
			Description: "Controls whether signup, password-reset, and email-change verification flows are available.",
			Environment: "AUTHARA_CHALLENGE_ENABLED", Control: ControlEnvironment, Reload: ReloadStartup,
			Group: "Challenge", Type: TypeBool, DefaultValue: "false", HasDefault: true, defaultValue: false,
			Impact: "Startup-only because this setting changes routes, rendered flows, and worker startup.",
		},
		{
			Key: KeyChallengeTTL, Name: "Challenge lifetime",
			Description: "Maximum lifetime assigned to newly created challenges.",
			Environment: "AUTHARA_CHALLENGE_TTL", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Challenge", Type: TypeDuration, DefaultValue: "30m", HasDefault: true, Minimum: "5m", Maximum: "24h",
			defaultValue: 30 * time.Minute, minDuration: durationPointer(5 * time.Minute), maxDuration: durationPointer(24 * time.Hour),
			Impact: "Affects newly created challenges only.",
		},
		{
			Key: KeyChallengeVerificationCodeTTL, Name: "Verification-code lifetime",
			Description: "Maximum lifetime assigned when a new verification code is generated.",
			Environment: "AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Challenge", Type: TypeDuration, DefaultValue: "10m", HasDefault: true, Minimum: "1m", Maximum: "1h",
			defaultValue: 10 * time.Minute, minDuration: durationPointer(time.Minute), maxDuration: durationPointer(time.Hour),
			Impact: "Affects newly generated codes only and never extends beyond the challenge expiry.",
		},
		{
			Key: KeyChallengeMaxAttempts, Name: "Maximum verification attempts",
			Description: "Attempt limit stored on newly created challenges.",
			Environment: "AUTHARA_CHALLENGE_MAX_ATTEMPTS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Challenge", Type: TypeInt, DefaultValue: "5", HasDefault: true, Minimum: "1", Maximum: "20",
			defaultValue: 5, minInt: intPointer(1), maxInt: intPointer(20),
			Impact: "Affects newly created challenges only.",
		},
		{
			Key: KeyChallengeMaxResends, Name: "Maximum resends",
			Description: "Resend limit stored on newly created challenges.",
			Environment: "AUTHARA_CHALLENGE_MAX_RESENDS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Challenge", Type: TypeInt, DefaultValue: "3", HasDefault: true, Minimum: "0", Maximum: "10",
			defaultValue: 3, minInt: intPointer(0), maxInt: intPointer(10),
			Impact: "Affects newly created challenges only.",
		},
		{
			Key: KeyChallengeMinimumResendInterval, Name: "Minimum resend interval",
			Description: "Minimum delay between sends, stored on newly created challenges.",
			Environment: "AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Challenge", Type: TypeDuration, DefaultValue: "30s", HasDefault: true, Minimum: "0s", Maximum: "15m",
			defaultValue: 30 * time.Second, minDuration: durationPointer(0), maxDuration: durationPointer(15 * time.Minute),
			Impact: "Affects newly created challenges only.",
		},
	}...)
	definitions = append(definitions, rateLimitDefinitions()...)
	return definitions
}()

func generalPolicyDefinitions() []Definition {
	return []Definition{
		{
			Key: KeyUIDefaultReturnTo, Name: "Default return path",
			Description: "Safe relative path used after authentication when no return_to value is supplied.",
			Environment: "AUTHARA_DEFAULT_RETURN_TO", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Public URL", Type: TypeString, DefaultValue: "/", HasDefault: true, defaultValue: "/",
			Impact: "Applies to subsequent requests that do not provide their own return_to value.",
		},
		{
			Key: KeyAuthenticationUsernameLoginEnabled, Name: "Username login",
			Description: "Allows password login using either an email address or username.",
			Environment: "AUTHARA_USERNAME_LOGIN_ENABLED", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Authentication", Type: TypeBool, DefaultValue: "false", HasDefault: true, defaultValue: false,
			Impact: "Applies to subsequent hosted and API password-login requests.",
		},
		{
			Key: KeyTokenAccessTTL, Name: "Access-token lifetime",
			Description: "Lifetime assigned to newly issued access tokens.",
			Environment: "AUTHARA_ACCESS_TOKEN_TTL_MINUTES", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Tokens", Type: TypeInt, DefaultValue: "10", HasDefault: true, Minimum: "1", Maximum: "1440",
			defaultValue: 10, minInt: intPointer(1), maxInt: intPointer(1440),
			Impact: "Applies to newly issued access tokens; existing tokens keep their expiry.",
		},
		{
			Key: KeySessionTTL, Name: "Session lifetime",
			Description: "Absolute lifetime assigned to newly created sessions, in days.",
			Environment: "AUTHARA_SESSION_TTL_DAYS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Sessions", Type: TypeInt, DefaultValue: "60", HasDefault: true, Minimum: "1", Maximum: "3650",
			defaultValue: 60, minInt: intPointer(1), maxInt: intPointer(3650),
			Impact: "Applies to newly created sessions; existing sessions keep their expiry.",
		},
		{
			Key: KeySessionRefreshTokenTTL, Name: "Refresh-token lifetime",
			Description: "Lifetime assigned to newly issued refresh tokens, in days.",
			Environment: "AUTHARA_REFRESH_TOKEN_TTL_DAYS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Sessions", Type: TypeInt, DefaultValue: "14", HasDefault: true, Minimum: "1", Maximum: "3650",
			defaultValue: 14, minInt: intPointer(1), maxInt: intPointer(3650),
			Impact: "Applies to newly issued refresh tokens; existing tokens keep their expiry.",
		},
		{
			Key: KeySessionRotation, Name: "Refresh-token rotation",
			Description: "Minimum age before rotating a refresh token; accepts off, always, or a Go duration.",
			Environment: "AUTHARA_REFRESH_TOKEN_ROTATION_INTERVAL", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Sessions", Type: TypeString, DefaultValue: "24h", HasDefault: true, defaultValue: "24h",
			Impact: "Applies the next time a refresh token is used.",
		},
		{
			Key: KeyOrganizationPublicManagementEnabled, Name: "Public organization management",
			Description: "Allows authenticated app clients to use public organization-management endpoints.",
			Environment: "AUTHARA_PUBLIC_ORGANIZATION_MANAGEMENT_ENABLED", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Organizations", Type: TypeBool, DefaultValue: "false", HasDefault: true, defaultValue: false,
			Impact: "Enables or disables access for subsequent public API requests; internal API access is unchanged.",
		},
		{
			Key: KeyOrganizationInvitationTTL, Name: "Organization invitation lifetime",
			Description: "Lifetime assigned to newly created or resent organization invitations.",
			Environment: "AUTHARA_ORGANIZATION_INVITATION_TTL", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Organizations", Type: TypeDuration, DefaultValue: "168h", HasDefault: true, Minimum: "5m", Maximum: "8760h",
			defaultValue: 168 * time.Hour, minDuration: durationPointer(5 * time.Minute), maxDuration: durationPointer(365 * 24 * time.Hour),
			Impact: "Applies to newly created or resent invitations; existing invitations keep their expiry.",
		},
		{
			Key: KeyAccessPolicyAllowlistEnabled, Name: "Email allowlist enforcement",
			Description: "Requires an email to be present in the allowlist before signup or authentication.",
			Environment: "AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Access policy", Type: TypeBool, DefaultValue: "false", HasDefault: true, defaultValue: false,
			Impact: "Applies immediately to subsequent signup, login, and session checks.",
		},
		{
			Key: KeyAdminAuditRetention, Name: "Admin audit retention",
			Description: "Number of days admin audit events are retained.",
			Environment: "AUTHARA_ADMIN_AUDIT_RETENTION_DAYS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Retention", Type: TypeInt, DefaultValue: "180", HasDefault: true, Minimum: "1", Maximum: "3650",
			defaultValue: 180, minInt: intPointer(1), maxInt: intPointer(3650),
			Impact: "Applies to the next cleanup run. Reducing it can delete older audit events.",
		},
		{
			Key: KeyEmailJobMaxAttempts, Name: "Email delivery attempts",
			Description: "Maximum attempts before an email job is marked failed.",
			Environment: "AUTHARA_EMAIL_JOB_MAX_ATTEMPTS", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Email", Type: TypeInt, DefaultValue: "10", HasDefault: true, Minimum: "1", Maximum: "100",
			defaultValue: 10, minInt: intPointer(1), maxInt: intPointer(100),
			Impact: "Applies when the next failed delivery attempt is evaluated, including existing jobs.",
		},
		{
			Key: KeyEmailCleanupSentAfter, Name: "Sent-email retention",
			Description: "How long successfully sent email jobs are retained.",
			Environment: "AUTHARA_EMAIL_CLEANUP_SENT_AFTER", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Retention", Type: TypeDuration, DefaultValue: "720h", HasDefault: true, Minimum: "1h", Maximum: "87600h",
			defaultValue: 720 * time.Hour, minDuration: durationPointer(time.Hour), maxDuration: durationPointer(10 * 365 * 24 * time.Hour),
			Impact: "Applies to the next cleanup run. Reducing it can delete older sent-job records.",
		},
		{
			Key: KeyEmailCleanupFailedAfter, Name: "Failed-email retention",
			Description: "How long permanently failed email jobs are retained.",
			Environment: "AUTHARA_EMAIL_CLEANUP_FAILED_AFTER", Control: ControlHybrid, Reload: ReloadDynamic,
			Group: "Retention", Type: TypeDuration, DefaultValue: "2160h", HasDefault: true, Minimum: "1h", Maximum: "87600h",
			defaultValue: 2160 * time.Hour, minDuration: durationPointer(time.Hour), maxDuration: durationPointer(10 * 365 * 24 * time.Hour),
			Impact: "Applies to the next cleanup run. Reducing it can delete older failed-job records.",
		},
		webhookCSVDefinition(),
		webhookDurationDefinition(KeyWebhookTimeout, "Webhook request timeout", "Maximum time allowed for one webhook delivery request.", "AUTHARA_WEBHOOK_TIMEOUT", 5*time.Second, 100*time.Millisecond, 5*time.Minute, "Applies to the next webhook delivery request."),
		webhookCountDefinition(KeyWebhookMaxDeliveryAttempts, "Webhook delivery attempts", "Maximum delivery attempts before a webhook event is marked failed.", "AUTHARA_WEBHOOK_MAX_DELIVERY_ATTEMPTS", 3, 1, 100, "Applies when the next failed delivery attempt is evaluated, including queued events."),
		webhookDurationDefinition(KeyWebhookProcessingStaleAfter, "Webhook processing timeout", "Age after which an in-progress webhook delivery is considered stale.", "AUTHARA_WEBHOOK_PROCESSING_STALE_AFTER", 2*time.Minute, time.Second, time.Hour, "Applies to the next stale-event reaper run."),
		webhookDurationDefinition(KeyWebhookDeliveredRetention, "Delivered-webhook retention", "How long successfully delivered webhook events are retained.", "AUTHARA_WEBHOOK_DELIVERED_RETENTION", 24*time.Hour, time.Hour, 365*24*time.Hour, "Applies to the next cleanup run. Reducing it can delete older delivery records."),
		webhookDurationDefinition(KeyWebhookFailedRetention, "Failed-webhook retention", "How long permanently failed webhook events are retained.", "AUTHARA_WEBHOOK_FAILED_RETENTION", 720*time.Hour, time.Hour, 10*365*24*time.Hour, "Applies to the next cleanup run. Reducing it can delete older failure records."),
		webhookCountDefinition(KeyWebhookMaintenanceBatchSize, "Webhook maintenance batch size", "Maximum events processed in one stale-reaper or cleanup batch.", "AUTHARA_WEBHOOK_MAINTENANCE_BATCH_SIZE", 1000, 1, 10000, "Applies to the next stale-reaper or cleanup batch."),
	}
}

func webhookCSVDefinition() Definition {
	return Definition{
		Key: KeyWebhookEnabledEvents, Name: "Enabled webhook events",
		Description: "Comma-separated event types to enqueue; an empty value enables all supported events.",
		Environment: "AUTHARA_WEBHOOK_ENABLED_EVENTS", Control: ControlHybrid, Reload: ReloadDynamic,
		Group: "Webhooks", Type: TypeCSV, DefaultValue: "", HasDefault: true, defaultValue: "",
		Impact: "Applies before subsequent webhook events are enqueued.",
	}
}

func webhookDurationDefinition(key Key, name, description, environment string, value, minimum, maximum time.Duration, impact string) Definition {
	return Definition{
		Key: key, Name: name, Description: description, Environment: environment,
		Control: ControlHybrid, Reload: ReloadDynamic, Group: "Webhooks", Type: TypeDuration,
		DefaultValue: value.String(), HasDefault: true, Minimum: minimum.String(), Maximum: maximum.String(), Impact: impact,
		defaultValue: value, minDuration: durationPointer(minimum), maxDuration: durationPointer(maximum),
	}
}

func webhookCountDefinition(key Key, name, description, environment string, value, minimum, maximum int, impact string) Definition {
	return Definition{
		Key: key, Name: name, Description: description, Environment: environment,
		Control: ControlHybrid, Reload: ReloadDynamic, Group: "Webhooks", Type: TypeInt,
		DefaultValue: strconv.Itoa(value), HasDefault: true, Minimum: strconv.Itoa(minimum), Maximum: strconv.Itoa(maximum), Impact: impact,
		defaultValue: value, minInt: intPointer(minimum), maxInt: intPointer(maximum),
	}
}

func rateLimitDefinitions() []Definition {
	return []Definition{
		rateLimitCountDefinition(KeyRateLimitLoginIPLimit, "Login attempts per IP", "AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT", 5, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitLoginIPWindow, "Login IP window", "AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW", time.Minute),
		rateLimitCountDefinition(KeyRateLimitLoginEmailLimit, "Login attempts per email or username", "AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT", 10, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitLoginEmailWindow, "Login email or username window", "AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW", time.Hour),

		rateLimitCountDefinition(KeyRateLimitSignupIPLimit, "Signup attempts per IP", "AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT", 3, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitSignupIPWindow, "Signup IP window", "AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW", time.Hour),
		rateLimitCountDefinition(KeyRateLimitSignupEmailLimit, "Signup attempts per email", "AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT", 3, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitSignupEmailWindow, "Signup email window", "AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW", 24*time.Hour),

		rateLimitCountDefinition(KeyRateLimitPasswordResetIPLimit, "Password-reset attempts per IP", "AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_LIMIT", 5, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitPasswordResetIPWindow, "Password-reset IP window", "AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW", time.Hour),
		rateLimitCountDefinition(KeyRateLimitPasswordResetEmailLimit, "Password-reset attempts per email", "AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_LIMIT", 3, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitPasswordResetEmailWindow, "Password-reset email window", "AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW", 24*time.Hour),

		rateLimitCountDefinition(KeyRateLimitPasskeyLoginIPLimit, "Passkey-login attempts per IP", "AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_LIMIT", 30, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitPasskeyLoginIPWindow, "Passkey-login IP window", "AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_WINDOW", 10*time.Minute),
		rateLimitCountDefinition(KeyRateLimitChallengeVerifyIPLimit, "Challenge-verification attempts per IP", "AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_LIMIT", 30, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitChallengeVerifyIPWindow, "Challenge-verification IP window", "AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW", 10*time.Minute),
		rateLimitCountDefinition(KeyRateLimitChallengeResendIPLimit, "Challenge-resend attempts per IP", "AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_LIMIT", 10, 1, 1_000_000),
		rateLimitWindowDefinition(KeyRateLimitChallengeResendIPWindow, "Challenge-resend IP window", "AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW", time.Hour),

		rateLimitCountDefinition(KeyRateLimitCleanupEvery, "In-memory cleanup frequency", "AUTHARA_RATE_LIMIT_CLEANUP_EVERY", 200, 1, 1_000_000),
		rateLimitCountDefinition(KeyRateLimitMaxEntries, "In-memory maximum entries", "AUTHARA_RATE_LIMIT_MAX_ENTRIES", 50_000, 100, 1_000_000),
	}
}

func rateLimitCountDefinition(key Key, name, environment string, defaultValue, minimum, maximum int) Definition {
	impact := "The new threshold applies to subsequent checks, including existing buckets."
	if key == KeyRateLimitCleanupEvery || key == KeyRateLimitMaxEntries {
		impact = "Applies to subsequent in-memory limiter calls and is unused by the Redis limiter."
	}
	return Definition{
		Key: key, Name: name,
		Description: "Typed rate-limiter policy.", Environment: environment,
		Control: ControlHybrid, Reload: ReloadDynamic, Group: "Rate limits", Type: TypeInt,
		DefaultValue: strconv.Itoa(defaultValue), HasDefault: true,
		Minimum: strconv.Itoa(minimum), Maximum: strconv.Itoa(maximum), Impact: impact,
		defaultValue: defaultValue, minInt: intPointer(minimum), maxInt: intPointer(maximum),
	}
}

func rateLimitWindowDefinition(key Key, name, environment string, defaultValue time.Duration) Definition {
	return Definition{
		Key: key, Name: name,
		Description: "Typed rate-limiter policy.", Environment: environment,
		Control: ControlHybrid, Reload: ReloadDynamic, Group: "Rate limits", Type: TypeDuration,
		DefaultValue: defaultValue.String(), HasDefault: true,
		Minimum: time.Second.String(), Maximum: (7 * 24 * time.Hour).String(),
		Impact:       "The new window applies when a bucket is next created; existing buckets keep their current reset time.",
		defaultValue: defaultValue, minDuration: durationPointer(time.Second), maxDuration: durationPointer(7 * 24 * time.Hour),
	}
}

func buildCatalog(variables []EnvironmentVariable) []Definition {
	runtimeByEnvironment := make(map[string]Definition, len(runtimeDefinitions))
	for _, definition := range runtimeDefinitions {
		runtimeByEnvironment[definition.Environment] = definition
	}

	out := make([]Definition, 0, len(variables))
	if len(variables) == 0 {
		out = append(out, runtimeDefinitions...)
		return out
	}
	includedRuntime := make(map[string]struct{}, len(runtimeDefinitions))
	for _, variable := range variables {
		if definition, ok := runtimeByEnvironment[variable.Name]; ok {
			definition.Required = variable.Required
			definition.HasDefault = definition.HasDefault || variable.HasDefault
			definition.Group = variable.Group
			out = append(out, definition)
			includedRuntime[variable.Name] = struct{}{}
			continue
		}

		definition := Definition{
			Key:          Key(variable.Name),
			Name:         variable.DisplayName,
			Environment:  variable.Name,
			Control:      ControlEnvironment,
			Reload:       ReloadStartup,
			Sensitive:    variable.Sensitive,
			Group:        variable.Group,
			Type:         ValueType(variable.Type),
			DefaultValue: variable.Default,
			Required:     variable.Required,
			HasDefault:   variable.HasDefault,
			defaultValue: variable.Default,
		}
		if variable.Name == "LOG_LEVEL" {
			definition.HasDefault = true
			definition.DefaultValue = "debug in dev; info in prod"
			definition.defaultValue = "debug"
		}
		out = append(out, definition)
	}
	for _, definition := range runtimeDefinitions {
		if _, ok := includedRuntime[definition.Environment]; !ok {
			out = append(out, definition)
		}
	}
	return out
}

func cloneCatalog(definitions []Definition) []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	for i := range out {
		out[i].Allowed = append([]string(nil), out[i].Allowed...)
	}
	return out
}

func definitionFor(key Key) (Definition, bool) {
	for _, definition := range runtimeDefinitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

func LookupDefinition(key Key) (Definition, bool) {
	definition, ok := definitionFor(key)
	if !ok {
		return Definition{}, false
	}
	definition.Allowed = append([]string(nil), definition.Allowed...)
	return definition, true
}
