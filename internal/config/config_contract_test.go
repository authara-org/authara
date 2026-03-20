package config

import (
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type configContract struct {
	Version   int                   `yaml:"version"`
	Variables []configContractEntry `yaml:"variables"`
}

type configContractEntry struct {
	Name       string `yaml:"name"`
	Stability  string `yaml:"stability"`
	Required   bool   `yaml:"required"`
	DefaultRaw any    `yaml:"default"`
}

type codeEnvSpec struct {
	Name     string
	Required bool
	Default  string
}

func TestConfigContract_StableVariablesMatchCode(t *testing.T) {
	contract := loadConfigContract(t)

	contractSpecs := stableContractSpecMap(t, contract)
	codeSpecs := codeEnvSpecMap(t)

	assertSameVariables(t, contractSpecs, codeSpecs)
	assertMatchingRequiredFlags(t, contractSpecs, codeSpecs)
	assertMatchingDefaults(t, contractSpecs, codeSpecs)
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
