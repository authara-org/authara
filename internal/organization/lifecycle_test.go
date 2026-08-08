package organization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

func TestRemoveOrganizationMemberProtectsLastMemberAndOwner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "lifecycle-owner@example.com", Username: "lifecycle-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Lifecycle", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti})

		err = svc.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{OrganizationID: org.ID, UserID: owner.ID, ActorUserID: owner.ID})
		if !errors.Is(err, ErrLastOrganizationMember) {
			t.Fatalf("expected last-member error, got %v", err)
		}

		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "lifecycle-member@example.com", Username: "lifecycle-member"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}

		err = svc.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{OrganizationID: org.ID, UserID: owner.ID, ActorUserID: owner.ID})
		if !errors.Is(err, ErrLastOrganizationOwner) {
			t.Fatalf("expected last-owner error, got %v", err)
		}
		if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, owner.ID); err != nil {
			t.Fatalf("expected owner membership to remain: %v", err)
		}
	})
}

func TestRemoveOrganizationMemberRevokesOrganizationSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "session-owner@example.com", Username: "session-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Session Org", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "session-member@example.com", Username: "session-member"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               member.ID,
			ActiveOrganizationID: org.ID,
			ExpiresAt:            time.Now().Add(time.Hour),
			UserAgent:            "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
			SessionID:      session.ID,
			OrganizationID: org.ID,
			TokenHash:      "lifecycle-refresh-token",
			ExpiresAt:      time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti})
		if err := svc.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{
			OrganizationID: org.ID,
			UserID:         member.ID,
			ActorUserID:    owner.ID,
		}); err != nil {
			t.Fatalf("RemoveOrganizationMember failed: %v", err)
		}
		if _, err := tdb.Store.GetSessionByID(ctx, session.ID); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("expected organization session deletion, got %v", err)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, "lifecycle-refresh-token"); !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("expected refresh-token deletion, got %v", err)
		}
	})
}

func TestRemoveOrganizationMemberUsesCutoffAfterOrganizationLock(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "cutoff-owner-" + suffix + "@example.com", Username: "cutoff-owner-" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "cutoff-member-" + suffix + "@example.com", Username: "cutoff-member-" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Cutoff", domain.OrganizationKindTeam)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
		OrganizationID: org.ID,
		UserID:         member.ID,
		Role:           domain.OrganizationRoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
		_ = tdb.Store.DeleteUser(context.Background(), member.ID)
		_ = tdb.Store.DeleteUser(context.Background(), owner.ID)
	})

	lockCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if _, err := tdb.Store.GetOrganizationByIDForUpdate(lockCtx, org.ID); err != nil {
		t.Fatal(err)
	}

	pub := &recordingPublisher{}
	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti, WebhookPublisher: pub})
	done := make(chan error, 1)
	go func() {
		done <- svc.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{
			OrganizationID: org.ID,
			UserID:         member.ID,
			ActorUserID:    owner.ID,
		})
	}()

	select {
	case err := <-done:
		t.Fatalf("member removal completed before organization lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	releasedAt := time.Now().UTC()
	if err := tdb.Tx.Commit(lockCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("member removal did not finish after organization lock was released")
	}

	evt := mustFindWebhookEvent(t, pub.events, webhook.EventOrganizationMembershipDeleted)
	if evt.CreatedAt.Before(releasedAt) {
		t.Fatalf("revocation cutoff %s predates lock release %s", evt.CreatedAt, releasedAt)
	}
}

func TestDeleteOrganizationDoesNotDeadlockInvitationAcceptance(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	owner, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "delete-invite-owner-" + suffix + "@example.com",
		Username: "delete-invite-owner-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	invitee, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "delete-invite-target-" + suffix + "@example.com",
		Username: "delete-invite-target-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Delete Invite", domain.OrganizationKindTeam)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti, InvitationTTL: time.Hour})
	invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
		OrganizationID: org.ID,
		ActorUserID:    owner.ID,
		Email:          invitee.Email,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
		_ = tdb.Store.DeleteUser(context.Background(), invitee.ID)
		_ = tdb.Store.DeleteUser(context.Background(), owner.ID)
	})

	lockCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if _, err := tdb.Store.GetOrganizationByIDForUpdate(lockCtx, org.ID); err != nil {
		t.Fatal(err)
	}

	acceptDone := make(chan error, 1)
	go func() {
		_, err := svc.AcceptInvitation(ctx, AcceptInvitationInput{
			RawToken: invite.RawToken,
			UserID:   invitee.ID,
			Now:      time.Now().UTC(),
		})
		acceptDone <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		probeCtx, probeCancel := context.WithTimeout(ctx, 50*time.Millisecond)
		_, probeErr := tdb.Store.GetUserByIDForUpdate(probeCtx, invitee.ID)
		probeCancel()
		if errors.Is(probeErr, context.DeadlineExceeded) {
			break
		}
		if probeErr != nil {
			t.Fatal(probeErr)
		}
		if time.Now().After(deadline) {
			t.Fatal("invitation acceptance did not acquire the user lock")
		}
	}

	if err := svc.DeleteOrganization(lockCtx, DeleteOrganizationInput{
		OrganizationID: org.ID,
		ActorUserID:    owner.ID,
	}); err != nil {
		t.Fatalf("DeleteOrganization failed while invitation acceptance was waiting: %v", err)
	}
	if err := tdb.Tx.Commit(lockCtx); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-acceptDone:
		if !errors.Is(err, store.ErrOrganizationNotFound) {
			t.Fatalf("expected acceptance to observe deleted organization, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("invitation acceptance did not finish after organization deletion")
	}
}

func TestPersonalOrganizationMembershipRemovalRules(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		creator, err := tdb.Store.CreateUser(ctx, domain.User{Email: "personal-creator@example.com", Username: "personal-creator"})
		if err != nil {
			t.Fatal(err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "personal-member@example.com", Username: "personal-member"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, creator.ID, "Personal", domain.OrganizationKindPersonal)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}

		personal := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModePersonal})
		err = personal.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{OrganizationID: org.ID, UserID: member.ID, ActorUserID: member.ID})
		if !errors.Is(err, ErrPersonalOrganizationImmutable) {
			t.Fatalf("expected personal mode to reject leaving, got %v", err)
		}

		multi := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti})
		if err := multi.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{OrganizationID: org.ID, UserID: member.ID, ActorUserID: member.ID}); err != nil {
			t.Fatalf("expected non-creator to leave in multi mode: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, member.ID); !errors.Is(err, store.ErrOrganizationMembershipNotFound) {
			t.Fatalf("expected member removal, got %v", err)
		}
		err = multi.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{OrganizationID: org.ID, UserID: creator.ID, ActorUserID: creator.ID})
		if !errors.Is(err, ErrPersonalOrganizationImmutable) {
			t.Fatalf("expected creator leave to remain forbidden, got %v", err)
		}
	})
}

func TestDeleteOrganizationRules(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		pub := &recordingPublisher{}
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-org-owner@example.com", Username: "delete-org-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Delete Org", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-org-member@example.com", Username: "delete-org-member"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}

		single := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeSingle})
		err = single.DeleteOrganization(ctx, DeleteOrganizationInput{OrganizationID: org.ID, ActorUserID: owner.ID})
		if !errors.Is(err, ErrOrganizationHasOtherMembers) {
			t.Fatalf("expected other-members error, got %v", err)
		}

		multi := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti, WebhookPublisher: pub})
		if err := multi.DeleteOrganization(ctx, DeleteOrganizationInput{OrganizationID: org.ID, ActorUserID: owner.ID}); err != nil {
			t.Fatalf("DeleteOrganization failed: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationByID(ctx, org.ID); !errors.Is(err, store.ErrOrganizationNotFound) {
			t.Fatalf("expected organization deletion, got %v", err)
		}
		if evt := mustFindWebhookEvent(t, pub.events, webhook.EventOrganizationDeleted); evt.Type != webhook.EventOrganizationDeleted {
			t.Fatalf("unexpected event: %+v", evt)
		}
	})
}

func TestDeletePersonalOrganizationIsForbidden(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "personal-owner@example.com", Username: "personal-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Personal", domain.OrganizationKindPersonal)
		if err != nil {
			t.Fatal(err)
		}
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti})
		err = svc.DeleteOrganization(ctx, DeleteOrganizationInput{OrganizationID: org.ID, ActorUserID: owner.ID})
		if !errors.Is(err, ErrPersonalOrganizationImmutable) {
			t.Fatalf("expected personal-organization error, got %v", err)
		}
	})
}

func TestTransferOrganizationOwnership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		pub := &recordingPublisher{}
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "transfer-owner@example.com", Username: "transfer-owner"})
		if err != nil {
			t.Fatal(err)
		}
		newOwner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "transfer-new-owner@example.com", Username: "transfer-new-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Transfer", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         newOwner.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti, WebhookPublisher: pub})
		if err := svc.TransferOrganizationOwnership(ctx, TransferOrganizationOwnershipInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			NewOwnerUserID: newOwner.ID,
		}); err != nil {
			t.Fatalf("TransferOrganizationOwnership failed: %v", err)
		}

		oldMembership, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		newMembership, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, newOwner.ID)
		if err != nil {
			t.Fatal(err)
		}
		if oldMembership.Role != domain.OrganizationRoleAdmin || newMembership.Role != domain.OrganizationRoleOwner {
			t.Fatalf("unexpected roles after transfer: old=%s new=%s", oldMembership.Role, newMembership.Role)
		}
		if err := svc.RemoveOrganizationMember(ctx, RemoveOrganizationMemberInput{
			OrganizationID: org.ID,
			UserID:         owner.ID,
			ActorUserID:    owner.ID,
		}); err != nil {
			t.Fatalf("former owner could not leave: %v", err)
		}
	})
}
