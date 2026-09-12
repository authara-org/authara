package config

import (
	"strconv"
	"strings"
	"time"
)

func intPointer(value int) *int { return &value }

func durationPointer(value time.Duration) *time.Duration { return &value }

var runtimeDefinitions = append([]Definition{
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
}, rateLimitDefinitions()...)

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
			definition.HasDefault = variable.HasDefault
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

func isRuntimePolicyKey(key Key) bool {
	value := string(key)
	return strings.HasPrefix(value, "challenge.") || strings.HasPrefix(value, "rate_limit.")
}

func LookupDefinition(key Key) (Definition, bool) {
	definition, ok := definitionFor(key)
	if !ok {
		return Definition{}, false
	}
	definition.Allowed = append([]string(nil), definition.Allowed...)
	return definition, true
}
