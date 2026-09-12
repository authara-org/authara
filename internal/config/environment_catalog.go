package config

import (
	"reflect"
	"strings"
	"time"
	"unicode"
)

// EnvironmentVariable describes one environment variable consumed by Config.
// The catalog is derived from the same struct tags used by go-envconfig so new
// variables cannot silently disappear from the operator settings overview.
type EnvironmentVariable struct {
	Name        string
	DisplayName string
	Group       string
	Type        string
	Default     string
	Required    bool
	HasDefault  bool
	Sensitive   bool
}

var sensitiveEnvironmentVariables = map[string]struct{}{
	"POSTGRESQL_USERNAME":         {},
	"POSTGRESQL_PASSWORD":         {},
	"AUTHARA_REDIS_PASSWORD":      {},
	"AUTHARA_JWT_KEYS":            {},
	"AUTHARA_INTERNAL_API_TOKEN":  {},
	"AUTHARA_WEBHOOK_SECRET":      {},
	"AUTHARA_EMAIL_SMTP_USERNAME": {},
	"AUTHARA_EMAIL_SMTP_PASSWORD": {},
}

var environmentVariableTypeOverrides = map[string]string{
	"APP_ENV":                "enum",
	"AUTHARA_CACHE_PROVIDER": "enum",
	"AUTHARA_ORG_MODE":       "enum",
	"AUTHARA_EMAIL_PROVIDER": "enum",
}

// EnvironmentVariables returns every environment variable in declaration
// order, grouped by the top-level Config section that owns it.
func EnvironmentVariables() []EnvironmentVariable {
	typeOfConfig := reflect.TypeOf(Config{})
	variables := make([]EnvironmentVariable, 0)
	for i := range typeOfConfig.NumField() {
		section := typeOfConfig.Field(i)
		collectEnvironmentVariables(section.Type, section.Name, &variables)
	}
	return variables
}

func collectEnvironmentVariables(typ reflect.Type, configSection string, out *[]EnvironmentVariable) {
	for i := range typ.NumField() {
		field := typ.Field(i)
		tag := field.Tag.Get("env")
		if tag == "" {
			if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
				collectEnvironmentVariables(field.Type, configSection, out)
			}
			continue
		}

		parts := strings.Split(tag, ",")
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		variable := EnvironmentVariable{
			Name:        name,
			DisplayName: environmentVariableDisplayName(name),
			Group:       environmentGroup(configSection, name),
			Type:        environmentValueType(field.Type, field.Name),
		}
		for _, rawOption := range parts[1:] {
			option := strings.TrimSpace(rawOption)
			switch {
			case option == "required":
				variable.Required = true
			case strings.HasPrefix(option, "default="):
				variable.HasDefault = true
				variable.Default = strings.TrimPrefix(option, "default=")
			}
		}
		_, variable.Sensitive = sensitiveEnvironmentVariables[name]
		if typeOverride, ok := environmentVariableTypeOverrides[name]; ok {
			variable.Type = typeOverride
		}
		*out = append(*out, variable)
	}
}

func environmentGroup(configSection, variableName string) string {
	switch configSection {
	case "Values":
		if variableName == "APP_ENV" {
			return "Runtime"
		}
		return "Public URL"
	case "Logging", "Observability":
		return "Runtime"
	case "UI":
		return "Public URL"
	case "DB":
		return "Database"
	case "OAuth":
		return "OAuth"
	case "Token":
		return "Tokens"
	case "Session":
		return "Sessions"
	case "RateLimit":
		return "Rate limits"
	case "Webhook":
		return "Webhooks"
	case "AccessPolicy":
		return "Access policy"
	case "Admin":
		return "Retention"
	case "InternalAPI":
		return "Internal API"
	case "Organization":
		return "Organizations"
	case "Challenge":
		return "Challenges"
	case "Email":
		if strings.HasPrefix(variableName, "AUTHARA_EMAIL_CLEANUP_") {
			return "Retention"
		}
		return "Email"
	default:
		return environmentDisplayName(configSection)
	}
}

func environmentValueType(typ reflect.Type, fieldName string) string {
	if typ == reflect.TypeOf(time.Duration(0)) || strings.HasSuffix(fieldName, "TTLRaw") || strings.HasSuffix(fieldName, "WindowRaw") {
		return "duration"
	}
	if strings.HasSuffix(fieldName, "URL") || strings.HasSuffix(fieldName, "URLRaw") {
		return "url"
	}
	switch typ.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Map:
		return "map"
	case reflect.Slice:
		return "csv"
	default:
		return "string"
	}
}

func environmentDisplayName(value string) string {
	words := splitIdentifier(value)
	for i, word := range words {
		switch strings.ToLower(word) {
		case "api", "db", "ip", "jwt", "oauth", "sql", "smtp", "tls", "ttl", "url":
			words[i] = strings.ToUpper(word)
		case "id":
			words[i] = "ID"
		default:
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

func environmentVariableDisplayName(name string) string {
	parts := strings.Split(name, "_")
	if len(parts) > 1 && parts[0] == "AUTHARA" {
		parts = parts[1:]
	}
	for i, part := range parts {
		switch part {
		case "API", "DB", "ID", "IP", "JWT", "OAUTH", "SQL", "SMTP", "TLS", "TTL", "URL":
			parts[i] = strings.ToUpper(part)
		case "POSTGRESQL":
			parts[i] = "PostgreSQL"
		default:
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func splitIdentifier(value string) []string {
	runes := []rune(value)
	if len(runes) == 0 {
		return nil
	}
	start := 0
	words := make([]string, 0, 4)
	for i := 1; i < len(runes); i++ {
		previousUpper := unicode.IsUpper(runes[i-1])
		currentUpper := unicode.IsUpper(runes[i])
		nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
		if (!previousUpper && currentUpper) || (previousUpper && currentUpper && nextLower) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	return append(words, string(runes[start:]))
}
