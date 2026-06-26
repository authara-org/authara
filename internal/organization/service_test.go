package organization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
)

type recordingPublisher struct {
	events []webhook.Envelope
}

func (p *recordingPublisher) Publish(ctx context.Context, evt webhook.Envelope) error {
	p.events = append(p.events, evt)
	return nil
}

func TestInvitationTokenHashing(t *testing.T) {
	raw, hash, err := generateInvitationToken()
	if err != nil {
		t.Fatalf("generateInvitationToken failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected raw token")
	}
	if hash == raw {
		t.Fatal("expected stored hash to differ from raw token")
	}

	got, err := hashInvitationToken(raw)
	if err != nil {
		t.Fatalf("hashInvitationToken failed: %v", err)
	}
	if got != hash {
		t.Fatalf("expected deterministic hash %q, got %q", hash, got)
	}
}

func TestNormalizeInvitationEmail(t *testing.T) {
	got, err := normalizeInvitationEmail(" Teammate@Example.COM ")
	if err != nil {
		t.Fatalf("normalizeInvitationEmail failed: %v", err)
	}
	if got != "teammate@example.com" {
		t.Fatalf("expected normalized email, got %q", got)
	}

	if _, err := normalizeInvitationEmail("not-an-email"); err == nil {
		t.Fatal("expected invalid email to fail")
	}
}

func TestInviteURLBuildsTokenURL(t *testing.T) {
	svc := New(Config{PublicURL: "https://auth.example.com"})

	got := svc.inviteURL("raw-token")
	want := "https://auth.example.com/auth/invitations/accept?token=raw-token"
	if got != want {
		t.Fatalf("expected invite URL %q, got %q", want, got)
	}
}

func TestOrganizationLifecycleWebhooks(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		pub := &recordingPublisher{}
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "org-webhook@example.com", Username: "org-webhook"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			Mode:             OrgModeMulti,
			WebhookPublisher: pub,
		})

		org, membership, err := svc.CreateOrganization(ctx, CreateOrganizationInput{
			Name:            "Webhook Org",
			CreatedByUserID: user.ID,
		})
		if err != nil {
			t.Fatalf("CreateOrganization failed: %v", err)
		}
		if _, err := svc.UpdateOrganization(ctx, org.ID, "Webhook Org Updated"); err != nil {
			t.Fatalf("UpdateOrganization failed: %v", err)
		}
		if _, err := svc.UpdateOrganizationMember(ctx, org.ID, membership.UserID, domain.OrganizationRoleAdmin); err != nil {
			t.Fatalf("UpdateOrganizationMember failed: %v", err)
		}
		if err := svc.DeleteOrganizationMember(ctx, org.ID, membership.UserID); err != nil {
			t.Fatalf("DeleteOrganizationMember failed: %v", err)
		}

		for _, eventType := range []webhook.EventType{
			webhook.EventOrganizationCreated,
			webhook.EventOrganizationMembershipCreated,
			webhook.EventOrganizationUpdated,
			webhook.EventOrganizationMembershipUpdated,
			webhook.EventOrganizationMembershipDeleted,
		} {
			mustFindWebhookEvent(t, pub.events, eventType)
		}
	})
}

func TestPersonalModeRejectsInvitations(t *testing.T) {
	svc := New(Config{Mode: OrgModePersonal})
	ctx := context.Background()

	if _, err := svc.CreateInvitation(ctx, CreateInvitationInput{
		Email: "invitee@example.com",
	}); !errors.Is(err, ErrOrganizationInviteForbidden) {
		t.Fatalf("expected ErrOrganizationInviteForbidden from CreateInvitation, got %v", err)
	}

	if _, err := svc.InvitationByToken(ctx, "token"); !errors.Is(err, ErrOrganizationInviteForbidden) {
		t.Fatalf("expected ErrOrganizationInviteForbidden from InvitationByToken, got %v", err)
	}

	if _, err := svc.AcceptInvitation(ctx, AcceptInvitationInput{
		RawToken: "token",
	}); !errors.Is(err, ErrOrganizationInviteForbidden) {
		t.Fatalf("expected ErrOrganizationInviteForbidden from AcceptInvitation, got %v", err)
	}
}

func TestResendInvitationRevokesOldAndCreatesFreshInvite(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "resend-owner@example.com", Username: "resend-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          OrgModeMulti,
			InvitationTTL: time.Hour,
			PublicURL:     "https://auth.example.com",
		})
		now := time.Now().UTC()
		first, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "resend-invitee@example.com",
			Now:            now,
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		resent, err := svc.ResendInvitation(ctx, ResendInvitationInput{
			OrganizationID: org.ID,
			InvitationID:   first.Invitation.ID,
			Now:            now.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("ResendInvitation failed: %v", err)
		}
		if resent.Invitation.ID == first.Invitation.ID {
			t.Fatal("expected resend to create a new invitation")
		}
		if resent.RawToken == "" || resent.RawToken == first.RawToken {
			t.Fatalf("expected fresh token, got %q", resent.RawToken)
		}
		if resent.InviteURL == "" {
			t.Fatal("expected fresh invite URL")
		}

		old, err := tdb.Store.GetOrganizationInvitationByID(ctx, first.Invitation.ID)
		if err != nil {
			t.Fatalf("GetOrganizationInvitationByID old failed: %v", err)
		}
		if old.Status(now.Add(time.Minute)) != domain.OrganizationInvitationStatusRevoked {
			t.Fatalf("expected old invitation revoked, got %+v", old)
		}
		if old.RevokedByUserID == nil || *old.RevokedByUserID != owner.ID {
			t.Fatalf("expected old invitation revoked by owner, got %+v", old.RevokedByUserID)
		}

		preview, err := svc.InvitationByToken(ctx, resent.RawToken)
		if err != nil {
			t.Fatalf("InvitationByToken resent failed: %v", err)
		}
		if preview.Invitation.ID != resent.Invitation.ID ||
			preview.Invitation.Email != first.Invitation.Email ||
			preview.Invitation.Role != first.Invitation.Role ||
			preview.Invitation.Status(now.Add(time.Minute)) != domain.OrganizationInvitationStatusPending {
			t.Fatalf("unexpected resent invitation: %+v", preview.Invitation)
		}
	})
}

func TestAcceptInvitationSingleModeRejectsExistingOtherMembership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "single-owner@example.com", Username: "single-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser owner failed: %v", err)
		}

		invitee, err := tdb.Store.CreateUser(ctx, domain.User{Email: "single-invitee@example.com", Username: "single-invitee"})
		if err != nil {
			t.Fatalf("CreateUser invitee failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, invitee.ID, invitee.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser invitee failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeSingle, InvitationTTL: time.Hour})
		now := time.Now().UTC()
		invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          invitee.Email,
			Now:            now,
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		_, err = svc.AcceptInvitation(ctx, AcceptInvitationInput{
			RawToken: invite.RawToken,
			UserID:   invitee.ID,
			Now:      now,
		})
		if !errors.Is(err, ErrOrganizationSingleMembershipConflict) {
			t.Fatalf("expected ErrOrganizationSingleMembershipConflict, got %v", err)
		}
	})
}

func TestAcceptInvitationMultiModeAllowsExistingOtherMembership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "multi-owner-accept@example.com", Username: "multi-owner-accept"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}

		invitee, err := tdb.Store.CreateUser(ctx, domain.User{Email: "multi-invitee-accept@example.com", Username: "multi-invitee-accept"})
		if err != nil {
			t.Fatalf("CreateUser invitee failed: %v", err)
		}
		if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, invitee.ID, invitee.Username); err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser invitee failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeMulti, InvitationTTL: time.Hour})
		now := time.Now().UTC()
		invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          invitee.Email,
			Now:            now,
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		result, err := svc.AcceptInvitation(ctx, AcceptInvitationInput{
			RawToken: invite.RawToken,
			UserID:   invitee.ID,
			Now:      now,
		})
		if err != nil {
			t.Fatalf("AcceptInvitation failed: %v", err)
		}
		if !result.InvitationAccepted || !result.MembershipCreated {
			t.Fatalf("expected invitation and membership creation, got %+v", result)
		}

		memberships, err := tdb.Store.ListOrganizationMembershipsByUserID(ctx, invitee.ID)
		if err != nil {
			t.Fatalf("ListOrganizationMembershipsByUserID failed: %v", err)
		}
		if len(memberships) != 2 {
			t.Fatalf("expected 2 memberships, got %d", len(memberships))
		}
	})
}

func mustFindWebhookEvent(t *testing.T, events []webhook.Envelope, eventType webhook.EventType) webhook.Envelope {
	t.Helper()

	for _, evt := range events {
		if evt.Type == eventType {
			return evt
		}
	}
	t.Fatalf("expected event %q in %+v", eventType, events)
	return webhook.Envelope{}
}
