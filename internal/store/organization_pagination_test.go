package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestOrganizationCollectionKeysetPagination(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	t.Run("members survive concurrent insert and delete", func(t *testing.T) {
		testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
			owner := createOrganizationStoreUser(t, ctx, tdb, "page-owner@example.com", "page-owner")
			org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
			if err != nil {
				t.Fatal(err)
			}
			base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
			for i := 0; i < 4; i++ {
				user := createOrganizationStoreUser(t, ctx, tdb, "page-member-"+string(rune('a'+i))+"@example.com", "page-member-"+string(rune('a'+i)))
				if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{OrganizationID: org.ID, UserID: user.ID, Role: domain.OrganizationRoleMember}); err != nil {
					t.Fatal(err)
				}
			}
			setMembershipTimes(t, ctx, org.ID, base)

			first, err := tdb.Store.ListOrganizationMembersPage(ctx, org.ID, nil, 2)
			if err != nil || len(first) != 2 {
				t.Fatalf("first page length = %d, err = %v", len(first), err)
			}
			cursor := &store.ListCursor{CreatedAt: first[1].Membership.CreatedAt, ID: first[1].Membership.UserID}

			all, err := tdb.Store.ListOrganizationMembersPage(ctx, org.ID, nil, 100)
			if err != nil {
				t.Fatal(err)
			}
			deleteID := all[2].Membership.UserID
			if err := tdb.Store.DeleteOrganizationMembership(ctx, org.ID, deleteID); err != nil {
				t.Fatal(err)
			}
			inserted := createOrganizationStoreUser(t, ctx, tdb, "page-member-new@example.com", "page-member-new")
			if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{OrganizationID: org.ID, UserID: inserted.ID, Role: domain.OrganizationRoleMember}); err != nil {
				t.Fatal(err)
			}
			execTestSQL(t, ctx, `UPDATE organization_memberships SET created_at = $3 WHERE organization_id = $1 AND user_id = $2`, org.ID, inserted.ID, base.Add(time.Hour))

			middle, err := tdb.Store.ListOrganizationMembersPage(ctx, org.ID, cursor, 2)
			if err != nil || len(middle) != 2 {
				t.Fatalf("middle page length = %d, err = %v", len(middle), err)
			}
			finalCursor := &store.ListCursor{CreatedAt: middle[1].Membership.CreatedAt, ID: middle[1].Membership.UserID}
			final, err := tdb.Store.ListOrganizationMembersPage(ctx, org.ID, finalCursor, 2)
			if err != nil || len(final) != 1 {
				t.Fatalf("final page length = %d, err = %v", len(final), err)
			}

			seen := map[uuid.UUID]bool{}
			for _, member := range append(append(first, middle...), final...) {
				if seen[member.User.ID] {
					t.Fatalf("duplicate member %s", member.User.ID)
				}
				seen[member.User.ID] = true
			}
			if seen[deleteID] {
				t.Fatalf("deleted member %s was returned", deleteID)
			}
			if !seen[inserted.ID] {
				t.Fatalf("concurrently inserted member %s was not returned", inserted.ID)
			}
		})
	})

	t.Run("memberships first middle and final", func(t *testing.T) {
		testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
			user := createOrganizationStoreUser(t, ctx, tdb, "page-memberships@example.com", "page-memberships")
			for i := 0; i < 5; i++ {
				org, err := tdb.Store.CreateOrganization(ctx, domain.Organization{Name: "Page team " + string(rune('A'+i)), Kind: domain.OrganizationKindTeam})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{OrganizationID: org.ID, UserID: user.ID, Role: domain.OrganizationRoleMember}); err != nil {
					t.Fatal(err)
				}
			}
			setUserMembershipTimes(t, ctx, user.ID, time.Date(2026, 9, 28, 13, 0, 0, 0, time.UTC))

			first, err := tdb.Store.ListUserOrganizationsPage(ctx, user.ID, nil, 2)
			if err != nil || len(first) != 2 {
				t.Fatalf("first page length = %d, err = %v", len(first), err)
			}
			middle, err := tdb.Store.ListUserOrganizationsPage(ctx, user.ID, &store.ListCursor{CreatedAt: first[1].Membership.CreatedAt, ID: first[1].Membership.OrganizationID}, 2)
			if err != nil || len(middle) != 2 {
				t.Fatalf("middle page length = %d, err = %v", len(middle), err)
			}
			final, err := tdb.Store.ListUserOrganizationsPage(ctx, user.ID, &store.ListCursor{CreatedAt: middle[1].Membership.CreatedAt, ID: middle[1].Membership.OrganizationID}, 10)
			if err != nil || len(final) != 1 {
				t.Fatalf("final page length = %d, err = %v", len(final), err)
			}
		})
	})

	t.Run("invitations empty first middle and final", func(t *testing.T) {
		testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
			owner := createOrganizationStoreUser(t, ctx, tdb, "page-invites@example.com", "page-invites")
			org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
			if err != nil {
				t.Fatal(err)
			}
			empty, err := tdb.Store.ListOrganizationInvitationsPage(ctx, org.ID, nil, 2)
			if err != nil || len(empty) != 0 {
				t.Fatalf("empty page length = %d, err = %v", len(empty), err)
			}
			for i := 0; i < 5; i++ {
				if _, err := tdb.Store.CreateOrganizationInvitation(ctx, domain.OrganizationInvitation{
					OrganizationID: org.ID, Email: "invite-" + string(rune('a'+i)) + "@example.com",
					Role: domain.OrganizationRoleMember, Metadata: json.RawMessage(`{}`),
					TokenHash: uuid.NewString(), InvitedByUserID: &owner.ID, ExpiresAt: time.Now().Add(time.Hour),
				}); err != nil {
					t.Fatal(err)
				}
			}
			setInvitationTimes(t, ctx, org.ID, time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC))

			first, err := tdb.Store.ListOrganizationInvitationsPage(ctx, org.ID, nil, 2)
			if err != nil || len(first) != 2 {
				t.Fatalf("first page length = %d, err = %v", len(first), err)
			}
			middle, err := tdb.Store.ListOrganizationInvitationsPage(ctx, org.ID, &store.ListCursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}, 2)
			if err != nil || len(middle) != 2 {
				t.Fatalf("middle page length = %d, err = %v", len(middle), err)
			}
			final, err := tdb.Store.ListOrganizationInvitationsPage(ctx, org.ID, &store.ListCursor{CreatedAt: middle[1].CreatedAt, ID: middle[1].ID}, 2)
			if err != nil || len(final) != 1 {
				t.Fatalf("final page length = %d, err = %v", len(final), err)
			}
		})
	})
}

func setMembershipTimes(t *testing.T, ctx context.Context, organizationID uuid.UUID, createdAt time.Time) {
	t.Helper()
	execTestSQL(t, ctx, `UPDATE organization_memberships SET created_at = $2 WHERE organization_id = $1`, organizationID, createdAt)
}

func setUserMembershipTimes(t *testing.T, ctx context.Context, userID uuid.UUID, createdAt time.Time) {
	t.Helper()
	execTestSQL(t, ctx, `UPDATE organization_memberships SET created_at = $2 WHERE user_id = $1`, userID, createdAt)
}

func setInvitationTimes(t *testing.T, ctx context.Context, organizationID uuid.UUID, createdAt time.Time) {
	t.Helper()
	execTestSQL(t, ctx, `UPDATE organization_invitations SET created_at = $2 WHERE organization_id = $1`, organizationID, createdAt)
}

func execTestSQL(t *testing.T, ctx context.Context, query string, args ...any) {
	t.Helper()
	txDB, ok := ctx.Value(store.DbKey).(*sql.Tx)
	if !ok {
		t.Fatal("expected transaction context")
	}
	if _, err := txDB.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}
