package config

import (
	"context"
	"net/http"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestUIConfigDefaultsUserUIToEnabled(t *testing.T) {
	var cfg UI
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   &cfg,
		Lookuper: envconfig.MapLookuper(nil),
	}); err != nil {
		t.Fatalf("process UI config: %v", err)
	}

	if !cfg.UserEnabled {
		t.Fatal("expected user-facing UI to be enabled by default")
	}
}

func TestErrorPageURLsValidateAndResolveByStatus(t *testing.T) {
	urls := ErrorPageURLs{
		BadRequest:          " https://app.example/errors/400 ",
		Unauthorized:        "https://app.example/errors/401",
		Forbidden:           "https://app.example/errors/403",
		NotFound:            "https://app.example/errors/404",
		Conflict:            "https://app.example/errors/409",
		Gone:                "https://app.example/errors/410",
		UnprocessableEntity: "https://app.example/errors/422",
		TooManyRequests:     "https://app.example/errors/429",
		InternalServerError: "https://app.example/errors/500",
	}
	if err := urls.validate(); err != nil {
		t.Fatal(err)
	}

	tests := map[int]string{
		http.StatusBadRequest:          urls.BadRequest,
		http.StatusUnauthorized:        urls.Unauthorized,
		http.StatusForbidden:           urls.Forbidden,
		http.StatusNotFound:            urls.NotFound,
		http.StatusConflict:            urls.Conflict,
		http.StatusGone:                urls.Gone,
		http.StatusUnprocessableEntity: urls.UnprocessableEntity,
		http.StatusTooManyRequests:     urls.TooManyRequests,
		http.StatusInternalServerError: urls.InternalServerError,
	}
	for status, want := range tests {
		if got := urls.URL(status); got != want {
			t.Errorf("URL(%d) = %q, want %q", status, got, want)
		}
	}
	if urls.URL(http.StatusServiceUnavailable) != "" {
		t.Fatal("unexpected custom page for an unsupported status")
	}
}

func TestErrorPageURLsRejectUnsafeDestinations(t *testing.T) {
	for _, target := range []string{"/errors/404", "ftp://app.example/errors/404", "https://user:pass@app.example/errors/404"} {
		urls := ErrorPageURLs{NotFound: target}
		if err := urls.validate(); err == nil {
			t.Errorf("validate target %q: expected error", target)
		}
	}
}

func TestUIConfigCanDisableUserUIFromEnvironment(t *testing.T) {
	var cfg UI
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target: &cfg,
		Lookuper: envconfig.MapLookuper(map[string]string{
			"AUTHARA_USER_UI_ENABLED": "false",
		}),
	}); err != nil {
		t.Fatalf("process UI config: %v", err)
	}

	if cfg.UserEnabled {
		t.Fatal("expected user-facing UI to be disabled")
	}
}
