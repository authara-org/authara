package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func TestRunOperationalCommandParsesSupportedCommands(t *testing.T) {
	tests := [][]string{
		{"operator", "grant", "--email", " User@Example.com "},
		{"operator", "revoke", "--email", " User@Example.com "},
		{"admin", "grant", "--email", " User@Example.com "},
		{"admin", "revoke", "--email", " User@Example.com "},
		{"allowlist", "add", "--email", " User@Example.com "},
		{"allowlist", "remove", "--email", " User@Example.com "},
	}

	for _, args := range tests {
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			var got operationalCommand
			var out bytes.Buffer
			err := runOperationalCommand(
				context.Background(),
				args,
				&out,
				func(_ context.Context, command operationalCommand) (string, error) {
					got = command
					return "completed", nil
				},
			)
			if err != nil {
				t.Fatalf("runOperationalCommand failed: %v", err)
			}
			if got.target != args[0] || got.action != args[1] || got.email != "user@example.com" {
				t.Fatalf("parsed command = %+v", got)
			}
			if out.String() != "completed\n" {
				t.Fatalf("output = %q, want completed", out.String())
			}
		})
	}
}

func TestRunOperationalCommandRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		nil,
		{"operator"},
		{"operator", "add", "--email", "user@example.com"},
		{"admin", "remove", "--email", "user@example.com"},
		{"allowlist", "grant", "--email", "user@example.com"},
		{"allowlist", "add"},
		{"allowlist", "add", "--email", "not-an-email"},
		{"allowlist", "add", "--email", "user@example.com", "extra"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			called := false
			err := runOperationalCommand(
				context.Background(),
				args,
				&bytes.Buffer{},
				func(context.Context, operationalCommand) (string, error) {
					called = true
					return "", nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), operationalCommandUsage) {
				t.Fatalf("error = %v, want usage error", err)
			}
			if called {
				t.Fatal("executor called for invalid arguments")
			}
		})
	}
}

func TestExecuteOperationalCommandGrantsRoles(t *testing.T) {
	tests := []struct {
		target   string
		roleName string
	}{
		{target: "operator", roleName: roles.DBOperatorRoleName},
		{target: "admin", roleName: roles.DBAdminRoleName},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			user := domain.User{ID: uuid.New(), Email: "user@example.com"}
			fake := &fakeOperationalStore{user: user}

			message, err := executeOperationalCommand(context.Background(), operationalDependencies{store: fake}, operationalCommand{
				target: tt.target,
				action: "grant",
				email:  user.Email,
			})
			if err != nil {
				t.Fatalf("executeOperationalCommand failed: %v", err)
			}
			if fake.addedRole != tt.roleName {
				t.Fatalf("added role = %q, want %q", fake.addedRole, tt.roleName)
			}
			if !strings.Contains(message, user.Email) || !strings.Contains(message, user.ID.String()) {
				t.Fatalf("message does not identify user: %q", message)
			}
		})
	}
}

func TestExecuteOperationalCommandRevokesRoleAndSessions(t *testing.T) {
	user := domain.User{ID: uuid.New(), Email: "operator@example.com"}
	fakeStore := &fakeOperationalStore{user: user, hasRole: true}
	fakeRevocations := &fakeUserAccessRevoker{}
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

	_, err := executeOperationalCommand(context.Background(), operationalDependencies{
		store:       fakeStore,
		tx:          inlineTransactionRunner{},
		revocations: fakeRevocations,
		now:         func() time.Time { return now },
	}, operationalCommand{target: "operator", action: "revoke", email: user.Email})
	if err != nil {
		t.Fatalf("executeOperationalCommand failed: %v", err)
	}
	if fakeStore.removedRole != roles.DBOperatorRoleName {
		t.Fatalf("removed role = %q, want %q", fakeStore.removedRole, roles.DBOperatorRoleName)
	}
	if fakeStore.revokedSessionsFor != user.ID {
		t.Fatalf("sessions revoked for %s, want %s", fakeStore.revokedSessionsFor, user.ID)
	}
	if fakeRevocations.userID != user.ID || !fakeRevocations.revokedAt.Equal(now) {
		t.Fatalf("access revocation = (%s, %s), want (%s, %s)", fakeRevocations.userID, fakeRevocations.revokedAt, user.ID, now)
	}
}

func TestExecuteOperationalCommandProtectsLastActiveAdmin(t *testing.T) {
	user := domain.User{ID: uuid.New(), Email: "admin@example.com"}
	fakeStore := &fakeOperationalStore{user: user, hasRole: true, activeRoleUsers: 1}

	_, err := executeOperationalCommand(context.Background(), operationalDependencies{
		store:       fakeStore,
		tx:          inlineTransactionRunner{},
		revocations: &fakeUserAccessRevoker{},
	}, operationalCommand{target: "admin", action: "revoke", email: user.Email})
	if !errors.Is(err, errLastActiveAdmin) {
		t.Fatalf("error = %v, want errLastActiveAdmin", err)
	}
	if !fakeStore.roleLocked {
		t.Fatal("admin role was not locked before checking the active admin count")
	}
	if fakeStore.removedRole != "" || fakeStore.revokedSessionsFor != uuid.Nil {
		t.Fatal("last active admin was modified")
	}
}

func TestExecuteOperationalCommandUpdatesAllowlistIdempotently(t *testing.T) {
	fake := &fakeOperationalStore{}

	_, err := executeOperationalCommand(context.Background(), operationalDependencies{store: fake}, operationalCommand{
		target: "allowlist",
		action: "add",
		email:  "user@example.com",
	})
	if err != nil {
		t.Fatalf("add allowlist email failed: %v", err)
	}
	if fake.allowedEmail != "user@example.com" {
		t.Fatalf("allowed email = %q", fake.allowedEmail)
	}

	fake.deleteAllowedEmailErr = store.ErrAllowedEmailNotFound
	_, err = executeOperationalCommand(context.Background(), operationalDependencies{store: fake}, operationalCommand{
		target: "allowlist",
		action: "remove",
		email:  "user@example.com",
	})
	if err != nil {
		t.Fatalf("remove missing allowlist email should be idempotent: %v", err)
	}
}

func TestExecuteOperationalCommandPreservesUserNotFound(t *testing.T) {
	fake := &fakeOperationalStore{getUserErr: store.ErrUserNotFound}

	_, err := executeOperationalCommand(context.Background(), operationalDependencies{store: fake}, operationalCommand{
		target: "operator",
		action: "grant",
		email:  "missing@example.com",
	})
	if !errors.Is(err, store.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
}

type inlineTransactionRunner struct{}

func (inlineTransactionRunner) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeUserAccessRevoker struct {
	userID    uuid.UUID
	revokedAt time.Time
}

func (f *fakeUserAccessRevoker) RevokeUser(_ context.Context, userID uuid.UUID, revokedAt time.Time) error {
	f.userID = userID
	f.revokedAt = revokedAt
	return nil
}

type fakeOperationalStore struct {
	user                  domain.User
	getUserErr            error
	addedRole             string
	removedRole           string
	hasRole               bool
	roleLocked            bool
	activeRoleUsers       int
	revokedSessionsFor    uuid.UUID
	allowedEmail          string
	deletedAllowedEmail   string
	deleteAllowedEmailErr error
}

func (f *fakeOperationalStore) GetUserByEmail(_ context.Context, _ string) (domain.User, error) {
	return f.user, f.getUserErr
}

func (f *fakeOperationalStore) AddUserPlatformRoleByName(_ context.Context, _ uuid.UUID, roleName string) error {
	f.addedRole = roleName
	return nil
}

func (f *fakeOperationalStore) RemoveUserPlatformRoleByName(_ context.Context, _ uuid.UUID, roleName string) error {
	f.removedRole = roleName
	return nil
}

func (f *fakeOperationalStore) UserHasPlatformRole(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return f.hasRole, nil
}

func (f *fakeOperationalStore) LockPlatformRoleByName(_ context.Context, _ string) error {
	f.roleLocked = true
	return nil
}

func (f *fakeOperationalStore) CountActiveUsersWithRole(_ context.Context, _ string) (int, error) {
	return f.activeRoleUsers, nil
}

func (f *fakeOperationalStore) RevokeAllSessionsForUser(_ context.Context, userID uuid.UUID, _ time.Time) error {
	f.revokedSessionsFor = userID
	return nil
}

func (f *fakeOperationalStore) EnsureAllowedEmail(_ context.Context, email string) error {
	f.allowedEmail = email
	return nil
}

func (f *fakeOperationalStore) DeleteAllowedEmail(_ context.Context, email string) error {
	f.deletedAllowedEmail = email
	return f.deleteAllowedEmailErr
}
