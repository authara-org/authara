package organization

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type RemoveOrganizationMemberInput struct {
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	ActorUserID    uuid.UUID
}

type DeleteOrganizationInput struct {
	OrganizationID uuid.UUID
	ActorUserID    uuid.UUID
}

type TransferOrganizationOwnershipInput struct {
	OrganizationID uuid.UUID
	ActorUserID    uuid.UUID
	NewOwnerUserID uuid.UUID
}

type lockedUserOrganization struct {
	organization domain.Organization
	membership   domain.OrganizationMembership
	memberships  []domain.OrganizationMembership
}

func (s *Service) RemoveOrganizationMember(ctx context.Context, in RemoveOrganizationMemberInput) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		org, err := s.store.GetOrganizationByIDForUpdate(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}
		if org.Kind == domain.OrganizationKindPersonal &&
			(s.mode != OrgModeMulti || org.CreatedByUserID == nil || *org.CreatedByUserID == in.UserID) {
			return ErrPersonalOrganizationImmutable
		}

		actor, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.ActorUserID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		target, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.UserID)
		if err != nil {
			return err
		}
		if in.ActorUserID != in.UserID && !canRemoveOrganizationMember(actor.Role, target.Role) {
			return ErrOrganizationActorNotAllowed
		}

		memberships, err := s.store.ListOrganizationMembershipsByOrganizationID(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}
		if len(memberships) == 1 {
			return ErrLastOrganizationMember
		}
		if target.Role == domain.OrganizationRoleOwner && countOrganizationOwners(memberships) == 1 {
			return ErrLastOrganizationOwner
		}

		return s.removeOrganizationMembership(txCtx, target, time.Now().UTC())
	})
}

func (s *Service) DeleteOrganization(ctx context.Context, in DeleteOrganizationInput) error {
	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		org, err := s.store.GetOrganizationByIDForUpdate(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}
		if org.Kind == domain.OrganizationKindPersonal {
			return ErrPersonalOrganizationImmutable
		}
		actor, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.ActorUserID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		if actor.Role != domain.OrganizationRoleOwner {
			return ErrOrganizationActorNotAllowed
		}

		memberships, err := s.store.ListOrganizationMembershipsByOrganizationID(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}
		if s.mode == OrgModeSingle && len(memberships) > 1 {
			return ErrOrganizationHasOtherMembers
		}
		return s.deleteOrganization(txCtx, org, memberships, time.Now().UTC())
	})
}

func (s *Service) TransferOrganizationOwnership(ctx context.Context, in TransferOrganizationOwnershipInput) error {
	if in.ActorUserID == in.NewOwnerUserID {
		return ErrInvalidOrganizationOwnershipTransfer
	}

	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.LockUserForKeyShare(txCtx, in.ActorUserID); err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		if err := s.store.LockUserForKeyShare(txCtx, in.NewOwnerUserID); err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				return store.ErrOrganizationMembershipNotFound
			}
			return err
		}

		org, err := s.store.GetOrganizationByIDForUpdate(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}
		if org.Kind == domain.OrganizationKindPersonal {
			return ErrPersonalOrganizationImmutable
		}

		actor, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.ActorUserID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		if actor.Role != domain.OrganizationRoleOwner {
			return ErrOrganizationActorNotAllowed
		}
		if _, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.NewOwnerUserID); err != nil {
			return err
		}

		newOwner, err := s.store.UpdateOrganizationMembershipRole(
			txCtx,
			in.OrganizationID,
			in.NewOwnerUserID,
			domain.OrganizationRoleOwner,
		)
		if err != nil {
			return err
		}
		oldOwner, err := s.store.UpdateOrganizationMembershipRole(
			txCtx,
			in.OrganizationID,
			in.ActorUserID,
			domain.OrganizationRoleAdmin,
		)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		for _, membership := range []domain.OrganizationMembership{newOwner, oldOwner} {
			if err := s.accessTokenRevocations.RevokeMembership(txCtx, membership.UserID, in.OrganizationID, now); err != nil {
				return err
			}
			if err := s.publish(txCtx, webhook.NewOrganizationMembershipUpdated(membership, now)); err != nil {
				return err
			}
		}
		return nil
	})
}

// PrepareUserDeletion removes organization-owned state without allowing user
// deletion to bypass last-owner and last-member rules. The caller must invoke
// it inside the same transaction that deletes the user.
func (s *Service) PrepareUserDeletion(ctx context.Context, userID uuid.UUID) error {
	memberships, err := s.store.ListOrganizationMembershipsByUserID(ctx, userID)
	if err != nil {
		return err
	}
	slices.SortFunc(memberships, func(a, b domain.OrganizationMembership) int {
		return slices.Compare(a.OrganizationID[:], b.OrganizationID[:])
	})

	locked := make([]lockedUserOrganization, 0, len(memberships))
	for _, membership := range memberships {
		org, err := s.store.GetOrganizationByIDForUpdate(ctx, membership.OrganizationID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationNotFound) {
				continue
			}
			return err
		}
		orgMemberships, err := s.store.ListOrganizationMembershipsByOrganizationID(ctx, org.ID)
		if err != nil {
			return err
		}
		membershipIndex := slices.IndexFunc(orgMemberships, func(candidate domain.OrganizationMembership) bool {
			return candidate.UserID == userID
		})
		if membershipIndex < 0 {
			continue
		}
		locked = append(locked, lockedUserOrganization{
			organization: org,
			membership:   orgMemberships[membershipIndex],
			memberships:  orgMemberships,
		})
	}

	for _, item := range locked {
		if item.organization.Kind == domain.OrganizationKindPersonal {
			if item.organization.CreatedByUserID == nil {
				return ErrPersonalOrganizationImmutable
			}
			if *item.organization.CreatedByUserID == userID {
				continue
			}
			if s.mode != OrgModeMulti {
				return ErrPersonalOrganizationImmutable
			}
		}
		if len(item.memberships) == 1 {
			return ErrLastOrganizationMember
		}
		if item.membership.Role == domain.OrganizationRoleOwner && countOrganizationOwners(item.memberships) == 1 {
			return ErrLastOrganizationOwner
		}
	}

	now := time.Now().UTC()
	for _, item := range locked {
		if item.organization.Kind == domain.OrganizationKindPersonal &&
			item.organization.CreatedByUserID != nil &&
			*item.organization.CreatedByUserID == userID {
			if err := s.deleteOrganization(ctx, item.organization, item.memberships, now); err != nil {
				return err
			}
			continue
		}
		if err := s.removeOrganizationMembership(ctx, item.membership, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) removeOrganizationMembership(ctx context.Context, membership domain.OrganizationMembership, now time.Time) error {
	if err := s.store.DeleteSessionsByOrganizationMembership(ctx, membership.OrganizationID, membership.UserID); err != nil {
		return err
	}
	if err := s.accessTokenRevocations.RevokeMembership(ctx, membership.UserID, membership.OrganizationID, now); err != nil {
		return err
	}
	if err := s.store.DeleteOrganizationMembership(ctx, membership.OrganizationID, membership.UserID); err != nil {
		return err
	}
	return s.publish(ctx, webhook.NewOrganizationMembershipDeleted(membership, now))
}

func (s *Service) deleteOrganization(ctx context.Context, org domain.Organization, memberships []domain.OrganizationMembership, now time.Time) error {
	if err := s.store.DeleteSessionsByOrganization(ctx, org.ID); err != nil {
		return err
	}
	for _, membership := range memberships {
		if err := s.accessTokenRevocations.RevokeMembership(ctx, membership.UserID, org.ID, now); err != nil {
			return err
		}
	}
	if err := s.store.DeleteOrganization(ctx, org.ID); err != nil {
		return err
	}
	return s.publish(ctx, webhook.NewOrganizationDeleted(org, now))
}

func canRemoveOrganizationMember(actorRole, targetRole domain.OrganizationRole) bool {
	return actorRole == domain.OrganizationRoleOwner ||
		(actorRole == domain.OrganizationRoleAdmin && targetRole != domain.OrganizationRoleOwner)
}

func countOrganizationOwners(memberships []domain.OrganizationMembership) int {
	owners := 0
	for _, membership := range memberships {
		if membership.Role == domain.OrganizationRoleOwner {
			owners++
		}
	}
	return owners
}
