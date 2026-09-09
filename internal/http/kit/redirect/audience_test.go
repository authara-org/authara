package redirect

import (
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/session/token"
)

func TestAudienceForPath(t *testing.T) {
	tests := []struct {
		path string
		want token.Audience
	}{
		{path: "/", want: token.AudienceApp},
		{path: "/auth/account", want: token.AudienceApp},
		{path: "/administrator", want: token.AudienceApp},
		{path: "/operators", want: token.AudienceApp},
		{path: "/admin", want: token.AudienceAdmin},
		{path: "/admin/users", want: token.AudienceAdmin},
		{path: "/auth/admin", want: token.AudienceAdmin},
		{path: "/auth/admin/users?query=user", want: token.AudienceAdmin},
		{path: "/operator", want: token.AudienceOperator},
		{path: "/operator/emails", want: token.AudienceOperator},
		{path: "/auth/operator", want: token.AudienceOperator},
		{path: "/auth/operator/emails#preview", want: token.AudienceOperator},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := AudienceForPath(tt.path); got != tt.want {
				t.Fatalf("AudienceForPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestAudienceFromRequest(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    token.Audience
		wantErr bool
	}{
		{name: "default", want: token.AudienceApp},
		{name: "app", raw: "app", want: token.AudienceApp},
		{name: "admin", raw: "admin", want: token.AudienceAdmin},
		{name: "operator", raw: "operator", want: token.AudienceOperator},
		{name: "invalid", raw: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/?audience="+tt.raw, nil)

			got, err := AudienceFromRequest(req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AudienceFromRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("AudienceFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
