package http

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestRouterAppliesForwardedIPOnlyWhenProxyTrustIsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		prefixes  []netip.Prefix
		wantLogIP string
	}{
		{
			name:      "enabled for trusted peer",
			enabled:   true,
			prefixes:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			wantLogIP: "client_ip=203.0.113.20",
		},
		{
			name:      "disabled despite configured network",
			prefixes:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			wantLogIP: "client_ip=10.0.0.5:1234",
		},
		{
			name:      "enabled without configured network",
			enabled:   true,
			wantLogIP: "client_ip=10.0.0.5:1234",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			router := NewRouter(ServerConfig{
				Version:                  "test",
				Logger:                   logger,
				Handlers:                 newTestHandlers(logger, nil),
				TrustProxyHeaders:        test.enabled,
				TrustedProxyCIDRs:        test.prefixes,
				disableOpenAPIValidation: true,
			}, passThroughMiddlewares())

			req := httptest.NewRequest(http.MethodGet, "/auth/live", nil)
			req.RemoteAddr = "10.0.0.5:1234"
			req.Header.Set("X-Forwarded-For", "203.0.113.20")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			if !strings.Contains(output.String(), test.wantLogIP) {
				t.Fatalf("logs = %q, want %q", output.String(), test.wantLogIP)
			}
		})
	}
}
