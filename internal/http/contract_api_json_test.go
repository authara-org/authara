package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/domain"
	authhandler "github.com/alexlup06-authgate/authgate/internal/http/handlers/auth"
	"github.com/alexlup06-authgate/authgate/internal/http/handlers/auth/api"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/httpctx"
	"github.com/alexlup06-authgate/authgate/internal/testutil"
)

func TestJSONContract_UserEndpointShape(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		createdUser, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "contract-user@example.com",
			Username: "contract-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		authSvc := auth.New(auth.Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		h := api.NewAPIHandler(authhandler.Deps{
			Auth: authSvc,
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/user", nil)
		req = req.WithContext(httpctx.WithUserID(ctx, createdUser.ID))

		rr := httptest.NewRecorder()
		h.UserGet(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		if got := rr.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %q", got)
		}

		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}

		checkStringField(t, body, "id")
		checkStringField(t, body, "username")
		checkStringField(t, body, "email")
		checkBoolField(t, body, "disabled")
		checkStringField(t, body, "created_at")
		checkArrayField(t, body, "roles")
	})
}

func checkStringField(t *testing.T, body map[string]any, field string) {
	t.Helper()

	v, ok := body[field]
	if !ok {
		t.Fatalf("missing required field %q", field)
	}

	if _, ok := v.(string); !ok {
		t.Fatalf("field %q must be string, got %T", field, v)
	}
}

func checkBoolField(t *testing.T, body map[string]any, field string) {
	t.Helper()

	v, ok := body[field]
	if !ok {
		t.Fatalf("missing required field %q", field)
	}

	if _, ok := v.(bool); !ok {
		t.Fatalf("field %q must be bool, got %T", field, v)
	}
}

func checkArrayField(t *testing.T, body map[string]any, field string) {
	t.Helper()

	v, ok := body[field]
	if !ok {
		t.Fatalf("missing required field %q", field)
	}

	if _, ok := v.([]any); !ok {
		t.Fatalf("field %q must be array, got %T", field, v)
	}
}
