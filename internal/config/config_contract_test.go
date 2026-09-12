package config

import (
	"bufio"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type configContract struct {
	Version          int                               `yaml:"version"`
	SettingsMetadata map[string]configSettingsMetadata `yaml:"settings_metadata"`
	Variables        []configContractEntry             `yaml:"variables"`
}

type configSettingsMetadata struct {
	Control   string `yaml:"control"`
	Reload    string `yaml:"reload"`
	Sensitive bool   `yaml:"sensitive"`
	Group     string `yaml:"group"`
}

type configContractEntry struct {
	Name       string `yaml:"name"`
	Stability  string `yaml:"stability"`
	Required   bool   `yaml:"required"`
	DefaultRaw any    `yaml:"default"`
}

type codeEnvSpec struct {
	Name       string
	Required   bool
	Default    string
	HasDefault bool
}

var dynamicHybridSettings = map[string]struct{}{
	"AUTHARA_CHALLENGE_TTL":                          {},
	"AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL":        {},
	"AUTHARA_CHALLENGE_MAX_ATTEMPTS":                 {},
	"AUTHARA_CHALLENGE_MAX_RESENDS":                  {},
	"AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL":          {},
	"AUTHARA_RATE_LIMIT_LOGIN_IP_LIMIT":              {},
	"AUTHARA_RATE_LIMIT_LOGIN_IP_WINDOW":             {},
	"AUTHARA_RATE_LIMIT_LOGIN_EMAIL_LIMIT":           {},
	"AUTHARA_RATE_LIMIT_LOGIN_EMAIL_WINDOW":          {},
	"AUTHARA_RATE_LIMIT_SIGNUP_IP_LIMIT":             {},
	"AUTHARA_RATE_LIMIT_SIGNUP_IP_WINDOW":            {},
	"AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_LIMIT":          {},
	"AUTHARA_RATE_LIMIT_SIGNUP_EMAIL_WINDOW":         {},
	"AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_LIMIT":     {},
	"AUTHARA_RATE_LIMIT_PASSWORD_RESET_IP_WINDOW":    {},
	"AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_LIMIT":  {},
	"AUTHARA_RATE_LIMIT_PASSWORD_RESET_EMAIL_WINDOW": {},
	"AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_LIMIT":      {},
	"AUTHARA_RATE_LIMIT_PASSKEY_LOGIN_IP_WINDOW":     {},
	"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_LIMIT":   {},
	"AUTHARA_RATE_LIMIT_CHALLENGE_VERIFY_IP_WINDOW":  {},
	"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_LIMIT":   {},
	"AUTHARA_RATE_LIMIT_CHALLENGE_RESEND_IP_WINDOW":  {},
	"AUTHARA_RATE_LIMIT_CLEANUP_EVERY":               {},
	"AUTHARA_RATE_LIMIT_MAX_ENTRIES":                 {},
}

func TestConfigContract_StableVariablesMatchCode(t *testing.T) {
	contract := loadConfigContract(t)

	contractSpecs := stableContractSpecMap(t, contract)
	codeSpecs := codeEnvSpecMap(t)

	assertSameVariables(t, contractSpecs, codeSpecs)
	assertMatchingRequiredFlags(t, contractSpecs, codeSpecs)
	assertMatchingDefaults(t, contractSpecs, codeSpecs)
	assertSettingsMetadata(t, contractSpecs, contract.SettingsMetadata)
}

func TestEnvironmentCatalogMatchesConfigAndSettingsMetadata(t *testing.T) {
	contract := loadConfigContract(t)
	codeSpecs := codeEnvSpecMap(t)
	variables := EnvironmentVariables()
	if len(variables) != len(codeSpecs) {
		t.Fatalf("environment catalog contains %d variables, want %d", len(variables), len(codeSpecs))
	}

	seen := make(map[string]struct{}, len(variables))
	for _, variable := range variables {
		if _, duplicate := seen[variable.Name]; duplicate {
			t.Fatalf("environment catalog contains duplicate %q", variable.Name)
		}
		seen[variable.Name] = struct{}{}

		spec, ok := codeSpecs[variable.Name]
		if !ok {
			t.Errorf("environment catalog contains unknown variable %q", variable.Name)
			continue
		}
		if variable.Required != spec.Required || variable.Default != spec.Default {
			t.Errorf("environment catalog %q required/default = %t/%q, want %t/%q", variable.Name, variable.Required, variable.Default, spec.Required, spec.Default)
		}
		if variable.HasDefault != spec.HasDefault {
			t.Errorf("environment catalog %q has-default = %t, want %t", variable.Name, variable.HasDefault, spec.HasDefault)
		}
		if variable.DisplayName == "" || variable.Group == "" || variable.Type == "" {
			t.Errorf("environment catalog %q has incomplete display metadata: %+v", variable.Name, variable)
		}

		metadata, ok := contract.SettingsMetadata[variable.Name]
		if !ok {
			t.Errorf("environment catalog %q has no settings metadata", variable.Name)
			continue
		}
		if variable.Sensitive != metadata.Sensitive {
			t.Errorf("environment catalog %q sensitive = %t, want %t", variable.Name, variable.Sensitive, metadata.Sensitive)
		}
		gotGroup := strings.ReplaceAll(strings.ToLower(variable.Group), " ", "_")
		if gotGroup != metadata.Group {
			t.Errorf("environment catalog %q group = %q, want %q", variable.Name, gotGroup, metadata.Group)
		}
	}
}

func TestEnvironmentExampleLeavesDynamicHybridSettingsUnset(t *testing.T) {
	file, err := os.Open("../../.env.example")
	if err != nil {
		t.Fatalf("open .env.example: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for name := range dynamicHybridSettings {
			if strings.HasPrefix(line, name+"=") {
				t.Errorf(".env.example actively sets hybrid variable %s, which locks its operator override", name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan .env.example: %v", err)
	}
}

func assertSettingsMetadata(
	t *testing.T,
	variables map[string]configContractEntry,
	metadata map[string]configSettingsMetadata,
) {
	t.Helper()
	if len(metadata) != len(variables) {
		t.Fatalf("settings metadata contains %d entries, want %d", len(metadata), len(variables))
	}
	for name := range variables {
		entry, ok := metadata[name]
		if !ok {
			t.Errorf("settings metadata is missing %q", name)
			continue
		}
		if entry.Control != "env" && entry.Control != "operator" && entry.Control != "hybrid" {
			t.Errorf("settings metadata %q has invalid control %q", name, entry.Control)
		}
		if entry.Reload != "startup" && entry.Reload != "dynamic" {
			t.Errorf("settings metadata %q has invalid reload %q", name, entry.Reload)
		}
		if entry.Group == "" {
			t.Errorf("settings metadata %q has an empty group", name)
		}
		if entry.Control == "env" && entry.Reload == "dynamic" {
			t.Errorf("settings metadata %q cannot be env-only and dynamic", name)
		}
		if entry.Sensitive && entry.Control != "env" {
			t.Errorf("sensitive settings metadata %q must remain environment-only", name)
		}
	}
	for name := range metadata {
		if _, ok := variables[name]; !ok {
			t.Errorf("settings metadata contains unknown variable %q", name)
		}
		_, wantDynamicHybrid := dynamicHybridSettings[name]
		entry := metadata[name]
		if gotDynamicHybrid := entry.Control == "hybrid" && entry.Reload == "dynamic"; gotDynamicHybrid != wantDynamicHybrid {
			t.Errorf("settings metadata %q dynamic hybrid classification = %t, want %t", name, gotDynamicHybrid, wantDynamicHybrid)
		}
	}
}

func loadConfigContract(t *testing.T) configContract {
	t.Helper()

	data, err := os.ReadFile("../../contract/config.yaml")
	if err != nil {
		t.Fatalf("read contract/config.yaml: %v", err)
	}

	var contract configContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/config.yaml: %v", err)
	}

	return contract
}

func stableContractSpecMap(t *testing.T, contract configContract) map[string]configContractEntry {
	t.Helper()

	out := make(map[string]configContractEntry, len(contract.Variables))

	for _, v := range contract.Variables {
		if v.Stability != "stable" {
			continue
		}
		if v.Name == "" {
			t.Fatal("contract/config.yaml contains stable variable with empty name")
		}
		if _, exists := out[v.Name]; exists {
			t.Fatalf("duplicate stable config variable in contract/config.yaml: %q", v.Name)
		}
		out[v.Name] = v
	}

	return out
}

func codeEnvSpecMap(t *testing.T) map[string]codeEnvSpec {
	t.Helper()

	out := make(map[string]codeEnvSpec)

	collectEnvSpecs(t, reflect.TypeOf(Config{}), out)

	return out
}

func collectEnvSpecs(t *testing.T, typ reflect.Type, out map[string]codeEnvSpec) {
	t.Helper()

	for i := range typ.NumField() {
		field := typ.Field(i)

		// Recurse into nested config structs.
		if field.Type.Kind() == reflect.Struct && field.Tag.Get("env") == "" {
			collectEnvSpecs(t, field.Type, out)
			continue
		}

		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}

		spec, ok := parseEnvTag(t, tag)
		if !ok {
			continue
		}

		if existing, exists := out[spec.Name]; exists {
			t.Fatalf(
				"duplicate env variable in config code: %q (existing=%+v, new=%+v)",
				spec.Name,
				existing,
				spec,
			)
		}

		out[spec.Name] = spec
	}
}

func parseEnvTag(t *testing.T, tag string) (codeEnvSpec, bool) {
	t.Helper()

	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return codeEnvSpec{}, false
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return codeEnvSpec{}, false
	}

	spec := codeEnvSpec{Name: name}

	for _, raw := range parts[1:] {
		part := strings.TrimSpace(raw)

		switch {
		case part == "required":
			spec.Required = true
		case strings.HasPrefix(part, "default="):
			spec.HasDefault = true
			spec.Default = strings.TrimPrefix(part, "default=")
		}
	}

	return spec, true
}

func assertSameVariables(
	t *testing.T,
	contractSpecs map[string]configContractEntry,
	codeSpecs map[string]codeEnvSpec,
) {
	t.Helper()

	var missingInCode []string
	for name := range contractSpecs {
		if _, ok := codeSpecs[name]; !ok {
			missingInCode = append(missingInCode, name)
		}
	}

	var missingInContract []string
	for name := range codeSpecs {
		if _, ok := contractSpecs[name]; !ok {
			missingInContract = append(missingInContract, name)
		}
	}

	sort.Strings(missingInCode)
	sort.Strings(missingInContract)

	if len(missingInCode) > 0 || len(missingInContract) > 0 {
		t.Fatalf(
			"config contract/code mismatch\nmissing in code: %v\nmissing in contract: %v",
			missingInCode,
			missingInContract,
		)
	}
}

func assertMatchingRequiredFlags(
	t *testing.T,
	contractSpecs map[string]configContractEntry,
	codeSpecs map[string]codeEnvSpec,
) {
	t.Helper()

	for name, contractSpec := range contractSpecs {
		codeSpec := codeSpecs[name]

		if contractSpec.Required != codeSpec.Required {
			t.Fatalf(
				"required mismatch for %q: contract=%v code=%v",
				name,
				contractSpec.Required,
				codeSpec.Required,
			)
		}
	}
}

func assertMatchingDefaults(
	t *testing.T,
	contractSpecs map[string]configContractEntry,
	codeSpecs map[string]codeEnvSpec,
) {
	t.Helper()

	for name, contractSpec := range contractSpecs {
		codeSpec := codeSpecs[name]

		contractDefault, hasContractDefault := normalizeContractDefault(contractSpec.DefaultRaw)
		codeDefault := codeSpec.Default
		hasCodeDefault := codeDefault != ""

		// Skip structured defaults like LOG_LEVEL's env-dependent default map.
		if isStructuredDefault(contractSpec.DefaultRaw) {
			continue
		}

		if hasContractDefault != hasCodeDefault {
			t.Fatalf(
				"default presence mismatch for %q: contract=%v code=%v",
				name,
				hasContractDefault,
				hasCodeDefault,
			)
		}

		if hasContractDefault && contractDefault != codeDefault {
			t.Fatalf(
				"default mismatch for %q: contract=%q code=%q",
				name,
				contractDefault,
				codeDefault,
			)
		}
	}
}

func normalizeContractDefault(v any) (string, bool) {
	if v == nil {
		return "", false
	}

	switch x := v.(type) {
	case string:
		return x, true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case float64:
		// YAML numbers often decode as float64
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10), true
		}
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return "", false
	}
}

func isStructuredDefault(v any) bool {
	switch v.(type) {
	case map[string]any, map[any]any, []any:
		return true
	default:
		return false
	}
}
