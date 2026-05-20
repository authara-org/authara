package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/features"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/testutil"
)

func TestAdminAllowlistResultsGetEmptyQueryReturnsFirstPage(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		for _, email := range []string{"allowlist-empty-alpha@example.com", "allowlist-empty-beta@example.com"} {
			if err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: email}); err != nil {
				t.Fatalf("CreateAllowedEmail failed: %v", err)
			}
		}

		body := adminAllowlistResultsBody(t, ctx, tdb, "/auth/admin/allowlist/results")
		if !strings.Contains(body, "allowlist-empty-alpha@example.com") || !strings.Contains(body, "allowlist-empty-beta@example.com") {
			t.Fatalf("expected empty query to render first page, got body=%s", body)
		}
	})
}

func TestAdminAllowlistResultsGetShortQueryReturnsMessage(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		if err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: "allowlist-short-alpha@example.com"}); err != nil {
			t.Fatalf("CreateAllowedEmail failed: %v", err)
		}

		body := adminAllowlistResultsBody(t, ctx, tdb, "/auth/admin/allowlist/results?q=al")
		if !strings.Contains(body, "Type at least 3 characters") {
			t.Fatalf("expected min-length message, got body=%s", body)
		}
		if strings.Contains(body, "allowlist-short-alpha@example.com") {
			t.Fatalf("short query should not render broad search results, got body=%s", body)
		}
	})
}

func TestAdminAllowlistResultsGetFiltersCaseInsensitive(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		for _, email := range []string{"allowlist-filter-alpha@example.com", "allowlist-filter-beta@example.com"} {
			if err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: email}); err != nil {
				t.Fatalf("CreateAllowedEmail failed: %v", err)
			}
		}

		body := adminAllowlistResultsBody(t, ctx, tdb, "/auth/admin/allowlist/results?q=FILTER-ALPHA")
		if !strings.Contains(body, "allowlist-filter-alpha@example.com") {
			t.Fatalf("expected case-insensitive filtered result, got body=%s", body)
		}
		if strings.Contains(body, "allowlist-filter-beta@example.com") {
			t.Fatalf("expected filtered result not to include beta, got body=%s", body)
		}
	})
}

func TestAdminAllowlistResultsGetDisabledReturnsNotFound(t *testing.T) {
	h := &UIHandler{
		Admin:    adminsvc.New(adminsvc.Config{}),
		Features: features.Features{AllowlistEnabled: false},
		Render:   render.New(render.Assets{}, false),
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist/results", nil)
	rr := httptest.NewRecorder()

	h.AdminAllowlistResultsGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func adminAllowlistResultsBody(t *testing.T, ctx context.Context, tdb *testutil.TestDB, target string) string {
	t.Helper()

	h := &UIHandler{
		Admin: adminsvc.New(adminsvc.Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			AllowlistEnabled: true,
		}),
		Features: features.Features{AllowlistEnabled: true},
		Render:   render.New(render.Assets{}, false),
	}
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	h.AdminAllowlistResultsGet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	return rr.Body.String()
}
