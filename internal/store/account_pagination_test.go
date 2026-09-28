package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestAccountCollectionPagination(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	t.Run("active sessions are stable across pages", func(t *testing.T) {
		testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
			user := createOrganizationStoreUser(t, ctx, tdb, "session-page@example.com", "session-page")
			org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC)
			for i := 0; i < 3; i++ {
				if _, err := tdb.Store.CreateSession(ctx, domain.Session{UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: now.Add(time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			execTestSQL(t, ctx, `UPDATE sessions SET created_at = $2 WHERE user_id = $1`, user.ID, now)

			first, err := tdb.Store.ListActiveSessionsPageByUserID(ctx, user.ID, now.Add(-time.Minute), nil, 2)
			if err != nil || len(first) != 2 {
				t.Fatalf("first page length = %d, err = %v", len(first), err)
			}
			final, err := tdb.Store.ListActiveSessionsPageByUserID(ctx, user.ID, now.Add(-time.Minute), &store.ListCursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}, 2)
			if err != nil || len(final) != 1 || final[0].ID == first[0].ID || final[0].ID == first[1].ID {
				t.Fatalf("final page = %+v, err = %v", final, err)
			}
		})
	})

	t.Run("passkeys are stable across concurrent changes", func(t *testing.T) {
		testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
			user := createStorePasskeyUser(t, ctx, tdb, "passkey-page@example.com", "passkey-page")
			base := time.Date(2026, 9, 28, 16, 0, 0, 0, time.UTC)
			for i := 0; i < 3; i++ {
				createStorePasskey(t, ctx, tdb, user.ID, "page-credential-"+string(rune('a'+i)))
			}
			execTestSQL(t, ctx, `UPDATE passkeys SET created_at = $2 WHERE user_id = $1`, user.ID, base)

			first, err := tdb.Store.ListPasskeysPageByUserID(ctx, user.ID, nil, 2)
			if err != nil || len(first) != 2 {
				t.Fatalf("first page length = %d, err = %v", len(first), err)
			}
			inserted := createStorePasskey(t, ctx, tdb, user.ID, "page-credential-new")
			execTestSQL(t, ctx, `UPDATE passkeys SET created_at = $2 WHERE id = $1`, inserted.ID, base.Add(time.Hour))
			final, err := tdb.Store.ListPasskeysPageByUserID(ctx, user.ID, &store.ListCursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}, 10)
			if err != nil || len(final) != 2 {
				t.Fatalf("final page length = %d, err = %v", len(final), err)
			}
			if final[0].ID == first[0].ID || final[0].ID == first[1].ID || final[1].ID == first[0].ID || final[1].ID == first[1].ID {
				t.Fatalf("duplicate passkey across pages: first=%+v final=%+v", first, final)
			}
		})
	})
}
