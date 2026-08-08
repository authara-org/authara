package organization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
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

func TestNormalizeInvitationRoleAndMetadata(t *testing.T) {
	role, err := normalizeInvitationRole("")
	if err != nil || role != domain.OrganizationRoleMember {
		t.Fatalf("expected default member role, got %q, %v", role, err)
	}
	role, err = normalizeInvitationRole(domain.OrganizationRoleAdmin)
	if err != nil || role != domain.OrganizationRoleAdmin {
		t.Fatalf("expected admin role, got %q, %v", role, err)
	}
	if _, err := normalizeInvitationRole(domain.OrganizationRoleOwner); !errors.Is(err, ErrInvalidOrganizationRole) {
		t.Fatalf("expected owner invitation role to fail, got %v", err)
	}

	metadata, err := normalizeInvitationMetadata(map[string]any{"product_role": "manager"})
	if err != nil || string(metadata) != `{"product_role":"manager"}` {
		t.Fatalf("unexpected metadata normalization: %s, %v", metadata, err)
	}
	if _, err := normalizeInvitationMetadata(map[string]any{"too_large": string(make([]byte, maxInvitationMetadataBytes))}); !errors.Is(err, ErrInvalidOrganizationInvitationMetadata) {
		t.Fatalf("expected oversized metadata to fail, got %v", err)
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
		otherOwner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "org-webhook-owner@example.com", Username: "org-webhook-owner"})
		if err != nil {
			t.Fatalf("CreateUser other owner failed: %v", err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         otherOwner.ID,
			Role:           domain.OrganizationRoleOwner,
		}); err != nil {
			t.Fatalf("CreateOrganizationMembership other owner failed: %v", err)
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
		created := mustFindWebhookEvent(t, pub.events, webhook.EventOrganizationMembershipCreated)
		data, ok := created.Data.(webhook.OrganizationMembershipCreatedData)
		if !ok || !data.IsInitialMembership || data.InvitationID != nil || string(data.Metadata) != "{}" {
			t.Fatalf("unexpected initial membership webhook data: %#v", created.Data)
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
			Role:           domain.OrganizationRoleAdmin,
			Metadata:       map[string]any{"product_role": "manager"},
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
		assertInvitationMetadataRole(t, preview.Invitation.Metadata, "manager")
	})
}

func TestCreateInvitationEmailCodeIsConfigurable(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	for _, tt := range []struct {
		name        string
		suffix      string
		includeCode bool
		wantCode    bool
	}{
		{name: "hidden by default", suffix: "hidden"},
		{name: "included when enabled", suffix: "included", includeCode: true, wantCode: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
				owner, err := tdb.Store.CreateUser(ctx, domain.User{
					Email:    "invite-code-owner-" + tt.suffix + "@example.com",
					Username: "invite-code-owner-" + tt.suffix,
				})
				if err != nil {
					t.Fatalf("CreateUser owner failed: %v", err)
				}
				org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
				if err != nil {
					t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
				}

				svc := New(Config{
					Store:              tdb.Store,
					Tx:                 tdb.Tx,
					Mode:               OrgModeMulti,
					InvitationTTL:      time.Hour,
					PublicURL:          "https://auth.example.com",
					IncludeCodeInEmail: tt.includeCode,
				})
				inviteeEmail := "invite-code-" + tt.suffix + "@example.com"
				invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
					OrganizationID: org.ID,
					ActorUserID:    owner.ID,
					Email:          inviteeEmail,
					Now:            time.Now().UTC(),
				})
				if err != nil {
					t.Fatalf("CreateInvitation failed: %v", err)
				}

				var data map[string]string
				if err := json.Unmarshal(loadInvitationEmailTemplateData(t, ctx, inviteeEmail), &data); err != nil {
					t.Fatalf("unmarshal template data: %v", err)
				}
				gotCode, hasCode := data["invitation_code"]
				if hasCode != tt.wantCode {
					t.Fatalf("expected invitation_code present=%v, got data %+v", tt.wantCode, data)
				}
				if tt.wantCode && gotCode != invite.RawToken {
					t.Fatalf("expected invitation code %q, got %q", invite.RawToken, gotCode)
				}
			})
		})
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

func TestAcceptInvitationSingleModeSerializesMembershipCreation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	invitee, err := tdb.Store.CreateUser(ctx, domain.User{Email: "single-concurrent-" + suffix + "@example.com", Username: "single-concurrent-" + suffix})
	if err != nil {
		t.Fatal(err)
	}

	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Mode: OrgModeSingle, InvitationTTL: time.Hour})
	owners := make([]domain.User, 0, 2)
	orgs := make([]domain.Organization, 0, 2)
	invites := make([]InvitationWithToken, 0, 2)
	for i := 0; i < 2; i++ {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "single-concurrent-owner-" + uuid.NewString() + "@example.com",
			Username: "single-concurrent-owner-" + uuid.NewString(),
		})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Concurrent", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          invitee.Email,
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		owners = append(owners, owner)
		orgs = append(orgs, org)
		invites = append(invites, invite)
	}
	t.Cleanup(func() {
		for _, org := range orgs {
			_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
		}
		_ = tdb.Store.DeleteUser(context.Background(), invitee.ID)
		for _, owner := range owners {
			_ = tdb.Store.DeleteUser(context.Background(), owner.ID)
		}
	})

	lockCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if _, err := tdb.Store.GetUserByIDForUpdate(lockCtx, invitee.ID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 2)
	for _, invite := range invites {
		invite := invite
		go func() {
			_, err := svc.AcceptInvitation(ctx, AcceptInvitationInput{RawToken: invite.RawToken, UserID: invitee.ID, Now: time.Now().UTC()})
			done <- err
		}()
	}
	select {
	case err := <-done:
		t.Fatalf("invitation acceptance completed before user lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tdb.Tx.Commit(lockCtx); err != nil {
		t.Fatal(err)
	}

	var succeeded, conflicted int
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, ErrOrganizationSingleMembershipConflict):
				conflicted++
			default:
				t.Fatalf("unexpected acceptance error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("invitation acceptance did not finish after user lock was released")
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("expected one success and one conflict, got success=%d conflict=%d", succeeded, conflicted)
	}
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

		pub := &recordingPublisher{}
		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			Mode:             OrgModeMulti,
			InvitationTTL:    time.Hour,
			WebhookPublisher: pub,
		})
		now := time.Now().UTC()
		invite, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          invitee.Email,
			Role:           domain.OrganizationRoleAdmin,
			Metadata:       map[string]any{"product_role": "manager"},
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
		if result.Membership.Role != domain.OrganizationRoleAdmin {
			t.Fatalf("expected admin membership, got %q", result.Membership.Role)
		}
		if _, err := svc.CreateInvitation(ctx, CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    invitee.ID,
			Email:          "next-manager@example.com",
			Role:           domain.OrganizationRoleAdmin,
			Now:            now,
		}); err != nil {
			t.Fatalf("expected invited admin to create another invitation: %v", err)
		}
		created := mustFindWebhookEvent(t, pub.events, webhook.EventOrganizationMembershipCreated)
		data, ok := created.Data.(webhook.OrganizationMembershipCreatedData)
		if !ok || data.IsInitialMembership || data.InvitationID == nil || *data.InvitationID != invite.Invitation.ID {
			t.Fatalf("unexpected invited membership webhook data: %#v", created.Data)
		}
		assertInvitationMetadataRole(t, data.Metadata, "manager")

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

func assertInvitationMetadataRole(t *testing.T, raw []byte, want string) {
	t.Helper()
	var metadata map[string]string
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("decode invitation metadata: %v", err)
	}
	if metadata["product_role"] != want {
		t.Fatalf("expected product_role %q, got %q", want, metadata["product_role"])
	}
}

func loadInvitationEmailTemplateData(t *testing.T, ctx context.Context, email string) []byte {
	t.Helper()

	db, ok := ctx.Value(store.DbKey).(*sql.Tx)
	if !ok {
		t.Fatal("expected transaction context")
	}

	var data []byte
	if err := db.QueryRowContext(ctx, `
		SELECT template_data
		FROM email_jobs
		WHERE to_email = $1 AND template = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, email, string(domain.EmailTemplateOrganizationInvite)).Scan(&data); err != nil {
		t.Fatalf("load invitation email job: %v", err)
	}
	return data
}
