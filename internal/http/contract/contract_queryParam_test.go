package contract

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	redir "github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/middleware"
	"gopkg.in/yaml.v3"
)

type httpContractQueryParam struct {
	QueryParameters []contractQueryParameter `yaml:"query_parameters"`
}

type contractQueryParameter struct {
	Name      string   `yaml:"name"`
	Stability string   `yaml:"stability"`
	AppliesTo []string `yaml:"applies_to"`
	Semantics []string `yaml:"semantics"`
}

func loadStableQueryParameter(t *testing.T, name string) contractQueryParameter {
	t.Helper()

	data, err := os.ReadFile("../../contract/http.yaml")
	if err != nil {
		t.Fatalf("read contract/http.yaml: %v", err)
	}

	var contract httpContractQueryParam
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/http.yaml: %v", err)
	}

	for _, qp := range contract.QueryParameters {
		if qp.Name == name && qp.Stability == "stable" {
			return qp
		}
	}

	t.Fatalf("stable query parameter %q not found in contract/http.yaml", name)
	return contractQueryParameter{}
}

func newReturnToHandler() http.Handler {
	return middleware.ReturnTo(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := httpctx.ReturnToOrDefault(r.Context(), "/")
		redir.Redirect(w, r, target, http.StatusSeeOther)
	}))
}

func TestRedirectContract_ReturnTo_AppliesToPaths(t *testing.T) {
	handler := newReturnToHandler()
	qp := loadStableQueryParameter(t, redirect.ReturnToQueryParam)

	if len(qp.AppliesTo) == 0 {
		t.Fatal("return_to contract has no applies_to paths")
	}

	for _, path := range qp.AppliesTo {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?"+redirect.ReturnToQueryParam+"=/private", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/private" {
				t.Fatalf("expected Location=/private, got %q", got)
			}

			if got := rr.Header().Get("HX-Redirect"); got != "/private" {
				t.Fatalf("expected HX-Redirect=/private, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_InvalidValueFallsBackToDefault(t *testing.T) {
	handler := newReturnToHandler()
	qp := loadStableQueryParameter(t, redirect.ReturnToQueryParam)

	for _, path := range qp.AppliesTo {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?return_to=//evil.com", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/" {
				t.Fatalf("expected Location=/, got %q", got)
			}

			if got := rr.Header().Get("HX-Redirect"); got != "/" {
				t.Fatalf("expected HX-Redirect=/, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_NoValueFallsBackToDefault(t *testing.T) {
	handler := newReturnToHandler()
	qp := loadStableQueryParameter(t, redirect.ReturnToQueryParam)

	for _, path := range qp.AppliesTo {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
			}

			if got := rr.Header().Get("Location"); got != "/" {
				t.Fatalf("expected Location=/, got %q", got)
			}

			if got := rr.Header().Get("HX-Redirect"); got != "/" {
				t.Fatalf("expected HX-Redirect=/, got %q", got)
			}
		})
	}
}

func TestRedirectContract_ReturnTo_HTMXUsesHXRedirect(t *testing.T) {
	handler := newReturnToHandler()
	qp := loadStableQueryParameter(t, redirect.ReturnToQueryParam)

	for _, path := range qp.AppliesTo {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path+"?"+redirect.ReturnToQueryParam+"=/private", nil)
			req.Header.Set("HX-Request", "true")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
			}

			if got := rr.Header().Get("HX-Redirect"); got != "/private" {
				t.Fatalf("expected HX-Redirect=/private, got %q", got)
			}

			if got := rr.Header().Get("Location"); got != "" {
				t.Fatalf("expected no Location header for HTMX redirect, got %q", got)
			}
		})
	}
}
