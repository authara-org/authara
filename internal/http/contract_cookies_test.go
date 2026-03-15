package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/session"
	"gopkg.in/yaml.v3"
)

type httpContractCookies struct {
	Version int              `yaml:"version"`
	Cookies []contractCookie `yaml:"cookies"`
}

type contractCookie struct {
	Name      string `yaml:"name"`
	Stability string `yaml:"stability"`
	Purpose   string `yaml:"purpose"`
}

func loadStableCookieNames(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile("../../contract/http.yaml")
	if err != nil {
		t.Fatalf("read contract/http.yaml: %v", err)
	}

	var contract httpContractCookies
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/http.yaml: %v", err)
	}

	var names []string
	for _, c := range contract.Cookies {
		if c.Stability == "stable" {
			names = append(names, c.Name)
		}
	}

	if len(names) == 0 {
		t.Fatal("no stable cookies found in contract/http.yaml")
	}

	return names
}

func cookieNameSet(cookies []*http.Cookie) map[string]bool {
	out := make(map[string]bool, len(cookies))
	for _, c := range cookies {
		out[c.Name] = true
	}
	return out
}

func requireCookiesPresent(t *testing.T, cookies []*http.Cookie, expected []string) {
	t.Helper()

	actual := cookieNameSet(cookies)

	for _, name := range expected {
		if !actual[name] {
			t.Fatalf("expected cookie %q to be set", name)
		}
	}
}

func TestStableCookieNames_AccessAndRefresh(t *testing.T) {
	rr := httptest.NewRecorder()

	session.SetAccessToken(rr, "access-token-value", 3600)
	session.SetRefreshToken(rr, "refresh-token-value", 7200)

	resp := rr.Result()

	requireCookiesPresent(t, resp.Cookies(), []string{
		"authara_access",
		"authara_refresh",
	})
}

func TestStableCookieNames_ClearSessionCookies(t *testing.T) {
	rr := httptest.NewRecorder()

	session.ClearSessionCookies(rr)

	resp := rr.Result()

	requireCookiesPresent(t, resp.Cookies(), []string{
		"authara_access",
		"authara_refresh",
	})
}

func TestStableCookiesFromContract(t *testing.T) {
	stableCookies := loadStableCookieNames(t)

	for _, name := range stableCookies {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			switch name {
			case "authara_access":
				session.SetAccessToken(rr, "access-token-value", 3600)

			case "authara_refresh":
				session.SetRefreshToken(rr, "refresh-token-value", 7200)

			case "authara_csrf":
				req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
				if _, err := csrf.EnsureCookie(rr, req); err != nil {
					t.Fatalf("EnsureCookie returned error: %v", err)
				}

			default:
				t.Fatalf("stable contract cookie %q has no producer test", name)
			}

			requireCookiesPresent(t, rr.Result().Cookies(), []string{name})
		})
	}
}
