package store_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestOrganizationStoreEnsureOrganizationMode(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		resetOrganizationMode(t, ctx)

		if err := tdb.Store.EnsureOrganizationMode(ctx, "single"); err != nil {
			t.Fatalf("EnsureOrganizationMode empty table failed: %v", err)
		}
		if err := tdb.Store.EnsureOrganizationMode(ctx, "single"); err != nil {
			t.Fatalf("EnsureOrganizationMode same mode failed: %v", err)
		}
		err := tdb.Store.EnsureOrganizationMode(ctx, "multi")
		if err == nil || !strings.Contains(err.Error(), "organization mode mismatch") {
			t.Fatalf("expected organization mode mismatch, got %v", err)
		}
	})
}

func TestOrganizationStoreDefaultOrganization(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createOrganizationStoreUser(t, ctx, tdb, "org-store@example.com", "org-store")

		org, membership, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		if org.Kind != domain.OrganizationKindPersonal {
			t.Fatalf("expected personal org, got %q", org.Kind)
		}
		if membership.Role != domain.OrganizationRoleOwner {
			t.Fatalf("expected owner membership, got %q", membership.Role)
		}

		orgAgain, membershipAgain, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, "different")
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser second call failed: %v", err)
		}
		if orgAgain.ID != org.ID || membershipAgain.OrganizationID != org.ID {
			t.Fatal("expected ensure to be idempotent")
		}

		memberships, err := tdb.Store.ListOrganizationMembershipsByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListOrganizationMembershipsByUserID failed: %v", err)
		}
		if len(memberships) != 1 {
			t.Fatalf("expected 1 membership, got %d", len(memberships))
		}

		_, err = tdb.Store.CreateOrganizationMembership(ctx, membership)
		if !store.IsUniqueViolation(err, "") {
			t.Fatalf("expected duplicate membership unique violation, got %v", err)
		}
	})
}

func TestOrganizationStoreMissingMembership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createOrganizationStoreUser(t, ctx, tdb, "org-missing@example.com", "org-missing")

		_, err := tdb.Store.GetOrganizationMembership(ctx, user.ID, user.ID)
		if !errors.Is(err, store.ErrOrganizationMembershipNotFound) {
			t.Fatalf("expected ErrOrganizationMembershipNotFound, got %v", err)
		}
	})
}

func TestOrganizationStoreRejectsBlankOrganizationName(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createOrganizationStoreUser(t, ctx, tdb, "org-blank@example.com", "org-blank")

		if _, err := tdb.Store.CreateOrganization(ctx, domain.Organization{
			Name: " ",
			Kind: domain.OrganizationKindTeam,
		}); !errors.Is(err, store.ErrInvalidOrganizationName) {
			t.Fatalf("expected ErrInvalidOrganizationName from CreateOrganization, got %v", err)
		}

		if _, _, err := tdb.Store.EnsureOrganizationForUser(ctx, user.ID, " ", domain.OrganizationKindTeam); !errors.Is(err, store.ErrInvalidOrganizationName) {
			t.Fatalf("expected ErrInvalidOrganizationName from EnsureOrganizationForUser, got %v", err)
		}
	})
}

func createOrganizationStoreUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email, username string) domain.User {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{Email: email, Username: username})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

func resetOrganizationMode(t *testing.T, ctx context.Context) {
	t.Helper()

	txDB, ok := ctx.Value(store.DbKey).(*sql.Tx)
	if !ok {
		t.Fatal("expected transaction context")
	}
	if _, err := txDB.ExecContext(ctx, `DELETE FROM organization_mode`); err != nil {
		t.Fatalf("reset organization_mode: %v", err)
	}
}
