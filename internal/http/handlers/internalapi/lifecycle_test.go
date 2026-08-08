package internalapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestRemoveInternalOrganizationMember(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-remove-owner@example.com", Username: "api-remove-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "API Remove", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-remove-member@example.com", Username: "api-remove-member"})
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

		handler := New(nil, organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti}), false)
		resp, err := handler.RemoveInternalOrganizationMember(ctx, contract.RemoveInternalOrganizationMemberRequestObject{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Body:           &contract.InternalOrganizationActorRequest{ActorUserId: owner.ID},
		})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
		}
		if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, member.ID); !errors.Is(err, store.ErrOrganizationMembershipNotFound) {
			t.Fatalf("expected membership removal, got %v", err)
		}
	})
}

func TestDeleteInternalOrganizationReturnsLifecycleConflict(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-delete-owner@example.com", Username: "api-delete-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "API Delete", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-delete-member@example.com", Username: "api-delete-member"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{OrganizationID: org.ID, UserID: member.ID, Role: domain.OrganizationRoleMember}); err != nil {
			t.Fatal(err)
		}

		handler := New(nil, organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle}), false)
		resp, err := handler.DeleteInternalOrganization(ctx, contract.DeleteInternalOrganizationRequestObject{
			OrganizationID: org.ID,
			Body:           &contract.InternalOrganizationActorRequest{ActorUserId: owner.ID},
		})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
		}
		var body contract.ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != string(codeOrganizationHasOtherMembers) {
			t.Fatalf("unexpected error: %+v", body)
		}
	})
}

func TestTransferInternalOrganizationOwnership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-transfer-owner@example.com", Username: "api-transfer-owner"})
		if err != nil {
			t.Fatal(err)
		}
		newOwner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-transfer-new-owner@example.com", Username: "api-transfer-new-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "API Transfer", domain.OrganizationKindTeam)
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

		handler := New(nil, organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti}), false)
		resp, err := handler.TransferInternalOrganizationOwnership(ctx, contract.TransferInternalOrganizationOwnershipRequestObject{
			OrganizationID: org.ID,
			Body: &contract.InternalOwnershipTransferRequest{
				ActorUserId:    owner.ID,
				NewOwnerUserId: newOwner.ID,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
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
	})
}

func TestDeleteInternalUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "api-delete-user@example.com", Username: "api-delete-user"})
		if err != nil {
			t.Fatal(err)
		}
		authService := auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx})
		handler := New(authService, nil, false)
		resp, err := handler.DeleteInternalUser(ctx, contract.DeleteInternalUserRequestObject{UserID: user.ID})
		if err != nil {
			t.Fatal(err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}
