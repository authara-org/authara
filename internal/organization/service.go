package organization

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type Config struct {
	Store            *store.Store
	Tx               *tx.Manager
	WebhookPublisher webhook.Publisher
	Logger           *slog.Logger
	InvitationTTL    time.Duration
	PublicURL        string
	Mode             OrgMode
}

type Service struct {
	store            *store.Store
	tx               *tx.Manager
	webhookPublisher webhook.Publisher
	logger           *slog.Logger
	invitationTTL    time.Duration
	publicURL        string
	mode             OrgMode
}

type UserOrganization struct {
	Organization domain.Organization
	Membership   domain.OrganizationMembership
}

type CreateOrganizationInput struct {
	Name            string
	CreatedByUserID uuid.UUID
}

func New(cfg Config) *Service {
	pub := cfg.WebhookPublisher
	if pub == nil {
		pub = webhook.NoopPublisher{}
	}

	return &Service{
		store:            cfg.Store,
		tx:               cfg.Tx,
		webhookPublisher: pub,
		logger:           cfg.Logger,
		invitationTTL:    cfg.InvitationTTL,
		publicURL:        strings.TrimRight(cfg.PublicURL, "/"),
		mode:             cfg.Mode,
	}
}

func (s *Service) EnsureDefaultOrganization(ctx context.Context, user domain.User) (domain.Organization, domain.OrganizationMembership, error) {
	org, membership, created, err := s.EnsureInitialOrganization(ctx, user, SignupSourceDirect)
	if err == nil && created {
		now := time.Now().UTC()
		s.publishBestEffort(ctx, webhook.NewOrganizationCreated(org, now))
		s.publishBestEffort(ctx, webhook.NewOrganizationMembershipCreated(membership, now))
	}
	return org, membership, err
}

func (s *Service) EnsureInitialOrganization(ctx context.Context, user domain.User, source SignupSource) (domain.Organization, domain.OrganizationMembership, bool, error) {
	var org domain.Organization
	var membership domain.OrganizationMembership
	plan, err := SignupOrganizationPlanFor(s.mode, source)
	if err != nil || !plan.CreateInitialOrg {
		return org, membership, false, err
	}

	created := false
	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		org, membership, created, err = s.store.EnsureOrganizationForUserWithCreated(
			txCtx,
			user.ID,
			defaultOrganizationName(user.Username, user.Email),
			plan.InitialOrgKind,
		)
		return err
	})
	return org, membership, created, err
}

func (s *Service) DefaultOrganizationForUser(ctx context.Context, userID uuid.UUID) (domain.Organization, domain.OrganizationMembership, error) {
	org, membership, err := s.store.GetPersonalOrganizationForUser(ctx, userID)
	if err == nil || !errors.Is(err, store.ErrOrganizationNotFound) {
		return org, membership, err
	}

	memberships, err := s.store.ListOrganizationMembershipsByUserID(ctx, userID)
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, err
	}
	if len(memberships) == 0 {
		return domain.Organization{}, domain.OrganizationMembership{}, store.ErrOrganizationNotFound
	}
	membership = memberships[0]
	org, err = s.store.GetOrganizationByID(ctx, membership.OrganizationID)
	return org, membership, err
}

func (s *Service) RequireMembership(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID) (domain.OrganizationMembership, error) {
	return s.store.GetOrganizationMembership(ctx, organizationID, userID)
}

func (s *Service) GetOrganization(ctx context.Context, organizationID uuid.UUID) (domain.Organization, error) {
	return s.store.GetOrganizationByID(ctx, organizationID)
}

func (s *Service) CreateOrganization(ctx context.Context, in CreateOrganizationInput) (domain.Organization, domain.OrganizationMembership, error) {
	if !s.mode.AllowsUserCreatedTeamOrgs() {
		return domain.Organization{}, domain.OrganizationMembership{}, ErrOrganizationOperationForbidden
	}
	if in.CreatedByUserID == uuid.Nil {
		return domain.Organization{}, domain.OrganizationMembership{}, store.ErrUserNotFound
	}

	var org domain.Organization
	var membership domain.OrganizationMembership
	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.store.GetUserByID(txCtx, in.CreatedByUserID); err != nil {
			return err
		}

		createdBy := in.CreatedByUserID
		var err error
		org, err = s.store.CreateOrganization(txCtx, domain.Organization{
			Name:            in.Name,
			Kind:            domain.OrganizationKindTeam,
			CreatedByUserID: &createdBy,
		})
		if err != nil {
			return err
		}

		membership, err = s.store.CreateOrganizationMembership(txCtx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         in.CreatedByUserID,
			Role:           domain.OrganizationRoleOwner,
		})
		return err
	})
	if err != nil {
		return domain.Organization{}, domain.OrganizationMembership{}, err
	}

	now := time.Now().UTC()
	s.publishBestEffort(ctx, webhook.NewOrganizationCreated(org, now))
	s.publishBestEffort(ctx, webhook.NewOrganizationMembershipCreated(membership, now))

	return org, membership, nil
}

func (s *Service) UpdateOrganization(ctx context.Context, organizationID uuid.UUID, name string) (domain.Organization, error) {
	if organizationID == uuid.Nil {
		return domain.Organization{}, store.ErrOrganizationNotFound
	}
	org, err := s.store.UpdateOrganizationName(ctx, organizationID, name)
	if err != nil {
		return domain.Organization{}, err
	}
	s.publishBestEffort(ctx, webhook.NewOrganizationUpdated(org, time.Now().UTC()))
	return org, nil
}

func (s *Service) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]UserOrganization, error) {
	memberships, err := s.store.ListOrganizationMembershipsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]UserOrganization, 0, len(memberships))
	for _, membership := range memberships {
		org, err := s.store.GetOrganizationByID(ctx, membership.OrganizationID)
		if err != nil {
			return nil, err
		}
		out = append(out, UserOrganization{Organization: org, Membership: membership})
	}
	return out, nil
}

func (s *Service) ListUserMemberships(ctx context.Context, userID uuid.UUID) ([]UserOrganization, error) {
	if _, err := s.store.GetUserByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.ListUserOrganizations(ctx, userID)
}

func (s *Service) ListOrganizationMembers(ctx context.Context, organizationID uuid.UUID) ([]domain.OrganizationMember, error) {
	if _, err := s.store.GetOrganizationByID(ctx, organizationID); err != nil {
		return nil, err
	}
	return s.store.ListOrganizationMembersByOrganizationID(ctx, organizationID)
}

func (s *Service) ListCurrentOrganizationMembers(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID) ([]domain.OrganizationMember, error) {
	if !s.mode.HasVisibleOrganizations() {
		return nil, ErrOrganizationOperationForbidden
	}
	if _, err := s.store.GetOrganizationMembership(ctx, organizationID, userID); err != nil {
		return nil, err
	}
	return s.ListOrganizationMembers(ctx, organizationID)
}

func (s *Service) GetOrganizationMember(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID) (domain.OrganizationMember, error) {
	if _, err := s.store.GetOrganizationByID(ctx, organizationID); err != nil {
		return domain.OrganizationMember{}, err
	}
	return s.store.GetOrganizationMember(ctx, organizationID, userID)
}

func (s *Service) UpdateOrganizationMember(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID, role domain.OrganizationRole) (domain.OrganizationMembership, error) {
	if !validOrganizationRole(role) {
		return domain.OrganizationMembership{}, ErrInvalidOrganizationRole
	}
	if _, err := s.store.GetOrganizationByID(ctx, organizationID); err != nil {
		return domain.OrganizationMembership{}, err
	}
	membership, err := s.store.UpdateOrganizationMembershipRole(ctx, organizationID, userID, role)
	if err != nil {
		return domain.OrganizationMembership{}, err
	}
	s.publishBestEffort(ctx, webhook.NewOrganizationMembershipUpdated(membership, time.Now().UTC()))
	return membership, nil
}

func (s *Service) DeleteOrganizationMember(ctx context.Context, organizationID uuid.UUID, userID uuid.UUID) error {
	if _, err := s.store.GetOrganizationByID(ctx, organizationID); err != nil {
		return err
	}
	membership, err := s.store.GetOrganizationMembership(ctx, organizationID, userID)
	if err != nil {
		return err
	}
	if err := s.store.DeleteOrganizationMembership(ctx, organizationID, userID); err != nil {
		return err
	}
	s.publishBestEffort(ctx, webhook.NewOrganizationMembershipDeleted(membership, time.Now().UTC()))
	return nil
}

func (s *Service) Mode() OrgMode {
	return s.mode
}

func (s *Service) SignupOrganizationPlan(source SignupSource) (SignupOrganizationPlan, error) {
	return SignupOrganizationPlanFor(s.mode, source)
}

func defaultOrganizationName(username, email string) string {
	if name := strings.TrimSpace(username); name != "" {
		return name
	}
	if local, _, ok := strings.Cut(strings.TrimSpace(email), "@"); ok && local != "" {
		return local
	}
	return "Personal workspace"
}

func validOrganizationRole(role domain.OrganizationRole) bool {
	switch role {
	case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleMember:
		return true
	default:
		return false
	}
}

func (s *Service) publishBestEffort(ctx context.Context, evt webhook.Envelope) {
	err := s.webhookPublisher.Publish(ctx, evt)
	if err != nil && s.logger != nil {
		s.logger.Error("webhook publish failed", "event", evt.Type, "event_id", evt.ID, "err", err)
	}
}
