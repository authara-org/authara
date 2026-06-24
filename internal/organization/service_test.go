package organization

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/testutil"
)

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

func TestInviteURLRejectsUnsafeReturnTo(t *testing.T) {
	svc := New(Config{PublicURL: "https://auth.example.com"})

	safe := svc.inviteURL("raw-token", "/settings/team")
	if !strings.Contains(safe, "return_to=%2Fsettings%2Fteam") {
		t.Fatalf("expected safe return_to in invite url, got %q", safe)
	}

	unsafe := svc.inviteURL("raw-token", "https://evil.example.com")
	if strings.Contains(unsafe, "return_to=") {
		t.Fatalf("expected unsafe return_to to be omitted, got %q", unsafe)
	}
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
