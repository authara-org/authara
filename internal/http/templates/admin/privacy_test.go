package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func TestUserDetailPrivacyRendering(t *testing.T) {
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	sessionID := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	fullUserAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

	html := renderAdminComponent(t, UserDetail(adminsvc.UserDetail{
		User: adminsvc.UserSummary{
			ID:        userID,
			CreatedAt: fixedTemplateTime(),
			UpdatedAt: fixedTemplateTime(),
			Username:  "privacy-user",
			Email:     "privacy-user@example.com",
			Roles:     []string{"admin"},
		},
		Passkeys: []adminsvc.PasskeySummary{{
			Name:           "Work laptop",
			CreatedAt:      fixedTemplateTime(),
			BackupEligible: true,
			DeviceLabel:    "Platform authenticator",
			Transport:      []string{"hybrid", "internal"},
		}},
		Sessions: []adminsvc.SessionSummary{{
			ID:               sessionID,
			CreatedAt:        fixedTemplateTime(),
			ExpiresAt:        fixedTemplateTime().Add(time.Hour),
			UserAgent:        fullUserAgent,
			UserAgentSummary: "Chrome on macOS",
			Status:           "Active",
		}},
		Actions: adminsvc.UserDetailActions{
			Disable:           adminsvc.ActionAvailability{Allowed: true},
			RevokeAdmin:       adminsvc.ActionAvailability{Allowed: true},
			RevokeAllSessions: adminsvc.ActionAvailability{Allowed: true},
		},
	}, FeatureFlags{AllowlistEnabled: true}))

	for _, want := range []string{
		"12345678",
		"Chrome on macOS",
		"Show full user agent",
		"Platform authenticator",
		"Show technical details",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected rendered user detail to contain %q", want)
		}
	}
	if strings.Contains(html, ">"+sessionID.String()+"<") {
		t.Fatal("full session id should not be rendered as visible table text")
	}
	for _, secret := range []string{"password-hash-secret", "refresh-token-hash-secret", "public-key-secret"} {
		if strings.Contains(html, secret) {
			t.Fatalf("secret material %q should not be rendered", secret)
		}
	}
}

func TestAuditPrivacyRendering(t *testing.T) {
	actorID := uuid.MustParse("9cf1b64b-0000-0000-0000-000000000000")
	targetID := uuid.MustParse("41ae92dc-0000-0000-0000-000000000000")
	targetEmail := "privacy-target@example.com"
	ip := "203.0.113.10"
	userAgent := "Full audit user agent"
	metadata, err := json.Marshal(map[string]any{
		"session_id":    "12345678-1234-1234-1234-123456789abc",
		"password_hash": "password-hash-secret",
	})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	html := renderAdminComponent(t, Audit(adminsvc.AuditEventPage{
		Events: []domain.AdminAuditEvent{{
			ID:           uuid.New(),
			CreatedAt:    fixedTemplateTime(),
			ActorUserID:  &actorID,
			Action:       "user.disabled",
			TargetUserID: &targetID,
			TargetEmail:  &targetEmail,
			Metadata:     metadata,
			IP:           &ip,
			UserAgent:    &userAgent,
		}},
		Page: 1,
		Size: 50,
	}, FeatureFlags{AllowlistEnabled: true}))

	for _, want := range []string{
		"9cf1b64b",
		"41ae92dc",
		"p***@example.com",
		"Show personal data",
		"privacy-target@example.com",
		"[redacted]",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected rendered audit page to contain %q", want)
		}
	}
	if strings.Contains(html, "password-hash-secret") {
		t.Fatal("audit metadata secrets should be redacted in the UI")
	}
}

func TestAllowlistLiveSearchMarkup(t *testing.T) {
	html := renderAdminComponent(t, Allowlist("alpha@example.com", FeatureFlags{AllowlistEnabled: true}))

	for _, want := range []string{
		`hx-get="/auth/admin/allowlist/results"`,
		`input changed delay:500ms`,
		`hx-target="#allowlist-results"`,
		`hx-swap="innerHTML"`,
		`hx-indicator="#allowlist-search-indicator"`,
		`id="allowlist-search-indicator"`,
		`id="allowlist-results"`,
		`Loading allowlist`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("expected allowlist markup to contain %q", want)
		}
	}
	if !strings.Contains(html, "keyup[key==&#39;Enter&#39;]") && !strings.Contains(html, "keyup[key=='Enter']") {
		t.Fatalf("expected allowlist search input to trigger on Enter, got html=%s", html)
	}
	if strings.Contains(html, "<table") {
		t.Fatal("allowlist page shell should not render the results table before the HTMX load")
	}
}

func TestDashboardHidesAllowlistWhenDisabled(t *testing.T) {
	html := renderAdminComponent(t, Dashboard(adminsvc.DashboardStats{}, adminsvc.RecentFailures{}, FeatureFlags{}))

	if strings.Contains(html, "/auth/admin/allowlist") || strings.Contains(html, ">Allowlist<") {
		t.Fatalf("allowlist UI should be hidden when disabled, got html=%s", html)
	}
}

func TestDashboardShowsAllowlistWhenEnabled(t *testing.T) {
	html := renderAdminComponent(t, Dashboard(adminsvc.DashboardStats{}, adminsvc.RecentFailures{}, FeatureFlags{AllowlistEnabled: true}))

	if !strings.Contains(html, "/auth/admin/allowlist") || !strings.Contains(html, ">Allowlist<") {
		t.Fatalf("allowlist UI should be rendered when enabled, got html=%s", html)
	}
}

func TestAllowlistResultsPaginationPreservesQuery(t *testing.T) {
	html := renderAdminComponent(t, AllowlistResults(adminsvc.AllowedEmailPage{
		Emails: []domain.AllowedEmail{{
			ID:        uuid.New(),
			CreatedAt: fixedTemplateTime(),
			Email:     "alpha@example.com",
		}},
		Query: "alpha",
		Page:  1,
		Size:  1,
		Total: 2,
	}))

	if !strings.Contains(html, "Showing 1-1 of 2 emails") {
		t.Fatalf("expected showing range, got html=%s", html)
	}
	if !strings.Contains(html, `/auth/admin/allowlist/results?page=2&amp;q=alpha`) &&
		!strings.Contains(html, `/auth/admin/allowlist/results?q=alpha&amp;page=2`) {
		t.Fatalf("expected next pagination link to preserve query, got html=%s", html)
	}
	if !strings.Contains(html, `hx-target="#allowlist-results"`) {
		t.Fatalf("expected pagination to target allowlist results, got html=%s", html)
	}
}

func renderAdminComponent(t *testing.T, c templ.Component) string {
	t.Helper()

	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render component: %v", err)
	}
	return buf.String()
}

func fixedTemplateTime() time.Time {
	return time.Date(2026, 5, 19, 10, 42, 0, 0, time.UTC)
}
