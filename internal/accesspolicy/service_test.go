package accesspolicy

import (
	"context"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/testutil"
)

func TestNoopEmailAccessPolicy_AllowsAnyEmail(t *testing.T) {
	policy := NoopEmailAccessPolicy{}

	allowed, err := policy.IsEmailAllowed(context.Background(), "anything@example.com")
	if err != nil {
		t.Fatalf("IsEmailAllowed returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected noop policy to allow any email")
	}
}

func TestService_IsEmailAllowed_DisabledAlwaysAllows(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:   tdb.Store,
			Enabled: false,
		})

		allowed, err := svc.IsEmailAllowed(ctx, "missing@example.com")
		if err != nil {
			t.Fatalf("IsEmailAllowed returned error: %v", err)
		}
		if !allowed {
			t.Fatal("expected disabled policy to allow any email")
		}
	})
}

func TestService_IsEmailAllowed_EnabledAllowsWhitelistedEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: "allowed@example.com"})
		if err != nil {
			t.Fatalf("CreateAllowedEmail failed: %v", err)
		}

		svc := New(Config{
			Store:   tdb.Store,
			Enabled: true,
		})

		allowed, err := svc.IsEmailAllowed(ctx, "allowed@example.com")
		if err != nil {
			t.Fatalf("IsEmailAllowed returned error: %v", err)
		}
		if !allowed {
			t.Fatal("expected whitelisted email to be allowed")
		}
	})
}

func TestService_IsEmailAllowed_EnabledDeniesMissingEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:   tdb.Store,
			Enabled: true,
		})

		allowed, err := svc.IsEmailAllowed(ctx, "missing@example.com")
		if err != nil {
			t.Fatalf("IsEmailAllowed returned error: %v", err)
		}
		if allowed {
			t.Fatal("expected non-whitelisted email to be denied")
		}
	})
}

func TestService_IsEmailAllowed_NormalizesEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: "normalized@example.com"})
		if err != nil {
			t.Fatalf("CreateAllowedEmail failed: %v", err)
		}

		svc := New(Config{
			Store:   tdb.Store,
			Enabled: true,
		})

		allowed, err := svc.IsEmailAllowed(ctx, "  NORMALIZED@EXAMPLE.COM  ")
		if err != nil {
			t.Fatalf("IsEmailAllowed returned error: %v", err)
		}
		if !allowed {
			t.Fatal("expected normalized email to be allowed")
		}
	})
}

func TestService_AllowEmail_AddsNormalizedEmailWhenEnabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:   tdb.Store,
			Enabled: true,
		})

		if err := svc.AllowEmail(ctx, "  Invited@Example.COM  "); err != nil {
			t.Fatalf("AllowEmail returned error: %v", err)
		}
		if err := svc.AllowEmail(ctx, "invited@example.com"); err != nil {
			t.Fatalf("AllowEmail should be idempotent, got: %v", err)
		}

		allowed, err := svc.IsEmailAllowed(ctx, "invited@example.com")
		if err != nil {
			t.Fatalf("IsEmailAllowed returned error: %v", err)
		}
		if !allowed {
			t.Fatal("expected allowed email")
		}
	})
}

func TestService_AllowEmail_DisabledDoesNothing(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:   tdb.Store,
			Enabled: false,
		})

		if err := svc.AllowEmail(ctx, "disabled@example.com"); err != nil {
			t.Fatalf("AllowEmail returned error: %v", err)
		}

		stored, err := tdb.Store.IsEmailAllowed(ctx, "disabled@example.com")
		if err != nil {
			t.Fatalf("store IsEmailAllowed returned error: %v", err)
		}
		if stored {
			t.Fatal("expected disabled allow policy not to create a row")
		}
	})
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercases", input: "USER@Example.COM", want: "user@example.com"},
		{name: "trims spaces", input: "  user@example.com  ", want: "user@example.com"},
		{name: "trims and lowercases", input: "  USER@Example.COM  ", want: "user@example.com"},
		{name: "empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize(tt.input)
			if got != tt.want {
				t.Fatalf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
