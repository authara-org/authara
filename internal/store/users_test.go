package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestCreateUserCanonicalizesIdentity(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "  Alice@Example.COM  ",
			Username: "Alice",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if user.Email != "alice@example.com" {
			t.Fatalf("email = %q, want canonical email", user.Email)
		}

		found, err := tdb.Store.GetUserByEmailOrUsername(ctx, " ALICE ")
		if err != nil {
			t.Fatalf("GetUserByEmailOrUsername failed: %v", err)
		}
		if found.ID != user.ID {
			t.Fatalf("found user %s, want %s", found.ID, user.ID)
		}
	})
}

func TestCreateUserCanonicalIdentityUniquenessIsRaceSafe(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	t.Run("email", func(t *testing.T) {
		suffix := uuid.NewString()
		users := []domain.User{
			{Email: "Race-" + suffix + "@Example.com", Username: "email-race-a-" + suffix},
			{Email: "race-" + suffix + "@example.COM", Username: "email-race-b-" + suffix},
		}
		assertConcurrentIdentityConflict(t, tdb, users, store.ConstraintUserEmail)
	})

	t.Run("username", func(t *testing.T) {
		suffix := uuid.NewString()
		users := []domain.User{
			{Email: "username-race-a-" + suffix + "@example.com", Username: "Race-" + suffix},
			{Email: "username-race-b-" + suffix + "@example.com", Username: "race-" + suffix},
		}
		assertConcurrentIdentityConflict(t, tdb, users, store.ConstraintUserUsername)
	})
}

func assertConcurrentIdentityConflict(t *testing.T, tdb *testutil.TestDB, users []domain.User, constraint string) {
	t.Helper()

	type result struct {
		user domain.User
		err  error
	}

	start := make(chan struct{})
	results := make(chan result, len(users))
	var wg sync.WaitGroup
	for _, candidate := range users {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			created, err := tdb.Store.CreateUser(context.Background(), candidate)
			results <- result{user: created, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for got := range results {
		switch {
		case got.err == nil:
			successes++
			if err := tdb.Store.DeleteUser(context.Background(), got.user.ID); err != nil {
				t.Fatalf("delete created user: %v", err)
			}
		case store.IsUniqueViolation(got.err, constraint):
			conflicts++
		default:
			t.Fatalf("unexpected create error: %v", got.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("results = %d successes, %d conflicts", successes, conflicts)
	}
}
