package http

import (
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/authara-org/authara/internal/http/handlers/auth/api"
	"github.com/authara-org/authara/internal/http/kit/response"
	"gopkg.in/yaml.v3"
)

type apiContract struct {
	JSONErrorContracts []apiErrorContract `yaml:"json_error_contracts"`
}

type apiErrorContract struct {
	Endpoint  string             `yaml:"endpoint"`
	Stability string             `yaml:"stability"`
	Errors    []apiContractError `yaml:"errors"`
}

type apiContractError struct {
	Status int    `yaml:"status"`
	Code   string `yaml:"code"`
}

func loadAPIContract(t *testing.T) apiContract {
	t.Helper()

	data, err := os.ReadFile("../../contract/api.yaml")
	if err != nil {
		t.Fatalf("read contract/api.yaml: %v", err)
	}

	var contract apiContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/api.yaml: %v", err)
	}

	return contract
}

func endpointKey(method, path string) string {
	return method + " " + path
}

func findErrorContract(t *testing.T, contract apiContract, endpoint string) apiErrorContract {
	t.Helper()

	for _, c := range contract.JSONErrorContracts {
		if c.Endpoint == endpoint && c.Stability == "stable" {
			return c
		}
	}

	t.Fatalf("stable json_error_contracts entry not found for %q", endpoint)
	return apiErrorContract{}
}

func errorKey(status int, code string) string {
	return fmt.Sprintf("%d:%s", status, code)
}

func contractErrorSet(errors []apiContractError) map[string]bool {
	out := make(map[string]bool, len(errors))
	for _, err := range errors {
		out[errorKey(err.Status, err.Code)] = true
	}
	return out
}

func routeErrorSet(errors map[response.ErrorCode]response.ErrorSpec) map[string]bool {
	out := make(map[string]bool, len(errors))
	for _, spec := range errors {
		out[errorKey(spec.Status, string(spec.Code))] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAPIContract_ErrorMappings(t *testing.T) {
	contract := loadAPIContract(t)

	for _, spec := range api.APIRouteSpecs {
		endpoint := endpointKey(spec.Method, spec.Path)

		t.Run(endpoint, func(t *testing.T) {
			wantContract := findErrorContract(t, contract, endpoint)

			want := contractErrorSet(wantContract.Errors)
			got := routeErrorSet(spec.Errors)

			wantKeys := sortedKeys(want)
			gotKeys := sortedKeys(got)

			if len(gotKeys) != len(wantKeys) {
				t.Fatalf(
					"error contract mismatch for %s\nwant: %v\ngot:  %v",
					endpoint,
					wantKeys,
					gotKeys,
				)
			}

			for _, key := range wantKeys {
				if !got[key] {
					t.Fatalf("missing error %q in implementation for %s", key, endpoint)
				}
			}

			for _, key := range gotKeys {
				if !want[key] {
					t.Fatalf("undeclared error %q in implementation for %s", key, endpoint)
				}
			}
		})
	}
}
