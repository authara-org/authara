package organization

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type CreateInvitationInput struct {
	OrganizationID uuid.UUID
	ActorUserID    uuid.UUID
	Email          string
	Now            time.Time
}

type InvitationWithToken struct {
	Invitation domain.OrganizationInvitation
	RawToken   string
	InviteURL  string
}

type InvitationPreview struct {
	Invitation   domain.OrganizationInvitation
	Organization domain.Organization
}

type AcceptInvitationInput struct {
	RawToken string
	UserID   uuid.UUID
	Now      time.Time
}

type AcceptInvitationByIDInput struct {
	InvitationID uuid.UUID
	UserID       uuid.UUID
	Now          time.Time
}

type AcceptInvitationResult struct {
	Invitation         domain.OrganizationInvitation
	Organization       domain.Organization
	Membership         domain.OrganizationMembership
	InvitationAccepted bool
	MembershipCreated  bool
}

func (s *Service) CreateInvitation(ctx context.Context, in CreateInvitationInput) (InvitationWithToken, error) {
	now := normalizeNow(in.Now)

	email, err := normalizeInvitationEmail(in.Email)
	if err != nil {
		return InvitationWithToken{}, err
	}
	if !s.mode.AllowsInvitations() {
		return InvitationWithToken{}, ErrOrganizationInviteForbidden
	}
	if in.ActorUserID == uuid.Nil {
		return InvitationWithToken{}, ErrOrganizationActorNotMember
	}

	rawToken, tokenHash, err := generateInvitationToken()
	if err != nil {
		return InvitationWithToken{}, err
	}
	inviteURL := s.inviteURL(rawToken)

	var out InvitationWithToken
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		org, err := s.store.GetOrganizationByID(txCtx, in.OrganizationID)
		if err != nil {
			return err
		}

		if _, err := s.store.GetUserByID(txCtx, in.ActorUserID); err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		membership, err := s.store.GetOrganizationMembership(txCtx, in.OrganizationID, in.ActorUserID)
		if err != nil {
			if errors.Is(err, store.ErrOrganizationMembershipNotFound) {
				return ErrOrganizationActorNotMember
			}
			return err
		}
		if !canInvite(membership.Role) {
			return ErrOrganizationInviteForbidden
		}

		target, err := s.store.GetUserByEmail(txCtx, email)
		if err != nil && !errors.Is(err, store.ErrUserNotFound) {
			return err
		}
		if err == nil {
			_, err = s.store.GetOrganizationMembership(txCtx, in.OrganizationID, target.ID)
			switch {
			case err == nil:
				return ErrOrganizationMemberAlreadyExists
			case errors.Is(err, store.ErrOrganizationMembershipNotFound):
			default:
				return err
			}
		}

		existing, err := s.store.GetActiveOrganizationInvitationByOrganizationAndEmail(txCtx, in.OrganizationID, email)
		if err != nil && !errors.Is(err, store.ErrOrganizationInvitationNotFound) {
			return err
		}
		if err == nil {
			if existing.ExpiresAt.After(now) {
				return ErrOrganizationInvitationAlreadyPending
			}
			if err := s.store.MarkOrganizationInvitationRevoked(txCtx, existing.ID, &in.ActorUserID, now); err != nil {
				return err
			}
		}

		invitation := domain.OrganizationInvitation{
			OrganizationID:  org.ID,
			Email:           email,
			Role:            domain.OrganizationRoleMember,
			TokenHash:       tokenHash,
			InvitedByUserID: &in.ActorUserID,
			ExpiresAt:       now.Add(s.invitationTTL),
		}

		created, err := s.store.CreateOrganizationInvitation(txCtx, invitation)
		if err != nil {
			if store.IsUniqueViolation(err, store.ConstraintActiveInvitation) {
				return ErrOrganizationInvitationAlreadyPending
			}
			return err
		}

		if err := s.enqueueInvitationEmail(txCtx, created, org, inviteURL, now); err != nil {
			return err
		}

		out = InvitationWithToken{
			Invitation: created,
			RawToken:   rawToken,
			InviteURL:  inviteURL,
		}
		return nil
	})
	if err != nil {
		return InvitationWithToken{}, err
	}

	s.publishBestEffort(ctx, webhook.NewOrganizationInvitationCreated(out.Invitation, now))

	return out, nil
}

func (s *Service) InvitationByToken(ctx context.Context, rawToken string) (InvitationPreview, error) {
	if !s.mode.AllowsInvitations() {
		return InvitationPreview{}, ErrOrganizationInviteForbidden
	}

	tokenHash, err := hashInvitationToken(strings.TrimSpace(rawToken))
	if err != nil {
		return InvitationPreview{}, err
	}

	invitation, err := s.store.GetOrganizationInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		return InvitationPreview{}, err
	}
	org, err := s.store.GetOrganizationByID(ctx, invitation.OrganizationID)
	if err != nil {
		return InvitationPreview{}, err
	}

	return InvitationPreview{
		Invitation:   invitation,
		Organization: org,
	}, nil
}

func (s *Service) InvitationByID(ctx context.Context, invitationID uuid.UUID) (InvitationPreview, error) {
	if !s.mode.AllowsInvitations() {
		return InvitationPreview{}, ErrOrganizationInviteForbidden
	}
	if invitationID == uuid.Nil {
		return InvitationPreview{}, store.ErrOrganizationInvitationNotFound
	}

	invitation, err := s.store.GetOrganizationInvitationByID(ctx, invitationID)
	if err != nil {
		return InvitationPreview{}, err
	}
	org, err := s.store.GetOrganizationByID(ctx, invitation.OrganizationID)
	if err != nil {
		return InvitationPreview{}, err
	}

	return InvitationPreview{
		Invitation:   invitation,
		Organization: org,
	}, nil
}

func (s *Service) AcceptInvitation(ctx context.Context, in AcceptInvitationInput) (AcceptInvitationResult, error) {
	if !s.mode.AllowsInvitations() {
		return AcceptInvitationResult{}, ErrOrganizationInviteForbidden
	}

	now := normalizeNow(in.Now)
	tokenHash, err := hashInvitationToken(strings.TrimSpace(in.RawToken))
	if err != nil {
		return AcceptInvitationResult{}, err
	}

	return s.acceptInvitation(ctx, in.UserID, now, func(txCtx context.Context) (domain.OrganizationInvitation, error) {
		return s.store.GetOrganizationInvitationByTokenHashForUpdate(txCtx, tokenHash)
	})
}

func (s *Service) AcceptInvitationByID(ctx context.Context, in AcceptInvitationByIDInput) (AcceptInvitationResult, error) {
	if !s.mode.AllowsInvitations() {
		return AcceptInvitationResult{}, ErrOrganizationInviteForbidden
	}
	if in.InvitationID == uuid.Nil {
		return AcceptInvitationResult{}, store.ErrOrganizationInvitationNotFound
	}

	now := normalizeNow(in.Now)
	return s.acceptInvitation(ctx, in.UserID, now, func(txCtx context.Context) (domain.OrganizationInvitation, error) {
		return s.store.GetOrganizationInvitationByIDForUpdate(txCtx, in.InvitationID)
	})
}

func (s *Service) acceptInvitation(
	ctx context.Context,
	userID uuid.UUID,
	now time.Time,
	loadInvitation func(context.Context) (domain.OrganizationInvitation, error),
) (AcceptInvitationResult, error) {
	var result AcceptInvitationResult
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		invitation, err := loadInvitation(txCtx)
		if err != nil {
			return err
		}

		user, err := s.store.GetUserByID(txCtx, userID)
		if err != nil {
			return err
		}
		userEmail, err := normalizeInvitationEmail(user.Email)
		if err != nil {
			return err
		}
		if userEmail != invitation.Email {
			return ErrOrganizationInviteEmailMismatch
		}

		org, err := s.store.GetOrganizationByID(txCtx, invitation.OrganizationID)
		if err != nil {
			return err
		}

		if invitation.Status(now) == domain.OrganizationInvitationStatusAccepted {
			if invitation.AcceptedByUserID != nil && *invitation.AcceptedByUserID == userID {
				membership, err := s.store.GetOrganizationMembership(txCtx, invitation.OrganizationID, userID)
				if err != nil {
					return err
				}
				result.Invitation = invitation
				result.Organization = org
				result.Membership = membership
				return nil
			}
			return ErrOrganizationInvitationAlreadyAccepted
		}
		switch invitation.Status(now) {
		case domain.OrganizationInvitationStatusRevoked:
			return ErrOrganizationInvitationRevoked
		case domain.OrganizationInvitationStatusExpired:
			return ErrOrganizationInvitationExpired
		}

		membership, err := s.store.GetOrganizationMembership(txCtx, invitation.OrganizationID, userID)
		switch {
		case err == nil:
		case errors.Is(err, store.ErrOrganizationMembershipNotFound):
			if s.mode == OrgModeSingle {
				memberships, err := s.store.ListOrganizationMembershipsByUserID(txCtx, userID)
				if err != nil {
					return err
				}
				if len(memberships) > 0 {
					return ErrOrganizationSingleMembershipConflict
				}
			}
			membership, err = s.store.CreateOrganizationMembership(txCtx, domain.OrganizationMembership{
				OrganizationID: invitation.OrganizationID,
				UserID:         userID,
				Role:           invitation.Role,
			})
			if err != nil {
				if store.IsUniqueViolation(err, "") {
					membership, err = s.store.GetOrganizationMembership(txCtx, invitation.OrganizationID, userID)
				}
				if err != nil {
					return err
				}
			} else {
				result.MembershipCreated = true
			}
		default:
			return err
		}

		if err := s.store.MarkOrganizationInvitationAccepted(txCtx, invitation.ID, userID, now); err != nil {
			return err
		}

		invitation.AcceptedAt = &now
		invitation.AcceptedByUserID = &userID

		result.Invitation = invitation
		result.Organization = org
		result.Membership = membership
		result.InvitationAccepted = true
		return nil
	})
	if err != nil {
		return AcceptInvitationResult{}, err
	}

	if result.InvitationAccepted {
		s.publishBestEffort(ctx, webhook.NewOrganizationInvitationAccepted(result.Invitation, now))
	}
	if result.MembershipCreated {
		s.publishBestEffort(ctx, webhook.NewOrganizationMembershipCreated(result.Membership, now))
	}

	return result, nil
}

func canInvite(role domain.OrganizationRole) bool {
	return role == domain.OrganizationRoleOwner || role == domain.OrganizationRoleAdmin
}

func normalizeInvitationEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", ErrInvalidOrganizationInvitationEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", ErrInvalidOrganizationInvitationEmail
	}
	return email, nil
}

func generateInvitationToken() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = base64.RawURLEncoding.EncodeToString(b)
	tokenHash, err = hashInvitationToken(rawToken)
	if err != nil {
		return "", "", err
	}
	return rawToken, tokenHash, nil
}

func hashInvitationToken(rawToken string) (string, error) {
	if rawToken == "" {
		return "", ErrInvalidOrganizationInvitationToken
	}
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:]), nil
}

func normalizeNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func (s *Service) inviteURL(rawToken string) string {
	base := s.publicURL
	if base == "" {
		base = "http://localhost:3000"
	}
	u, _ := url.Parse(base + "/auth/invitations/accept")
	q := u.Query()
	q.Set("token", rawToken)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Service) enqueueInvitationEmail(ctx context.Context, invitation domain.OrganizationInvitation, org domain.Organization, inviteURL string, now time.Time) error {
	data, err := json.Marshal(map[string]string{
		"organization_name": org.Name,
		"invite_url":        inviteURL,
		"role":              string(invitation.Role),
		"expires_at":        invitation.ExpiresAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal invitation email template data: %w", err)
	}

	_, err = s.store.CreateEmailJob(ctx, domain.EmailJob{
		ToEmail:       invitation.Email,
		Template:      domain.EmailTemplateOrganizationInvite,
		TemplateData:  data,
		Status:        domain.EmailJobStatusPending,
		NextAttemptAt: now,
	})
	return err
}
