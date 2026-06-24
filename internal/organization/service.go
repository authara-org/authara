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
	org, membership, _, err := s.EnsureInitialOrganization(ctx, user, SignupSourceDirect)
	return org, membership, err
}

func (s *Service) EnsureInitialOrganization(ctx context.Context, user domain.User, source SignupSource) (domain.Organization, domain.OrganizationMembership, bool, error) {
	var org domain.Organization
	var membership domain.OrganizationMembership
	plan, err := SignupOrganizationPlanFor(s.mode, source)
	if err != nil || !plan.CreateInitialOrg {
		return org, membership, false, err
	}

	err = s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		var err error
		org, membership, err = s.store.EnsureOrganizationForUser(
			txCtx,
			user.ID,
			defaultOrganizationName(user.Username, user.Email),
			plan.InitialOrgKind,
		)
		return err
	})
	return org, membership, true, err
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

func (s *Service) publishBestEffort(ctx context.Context, evt webhook.Envelope) {
	err := s.webhookPublisher.Publish(ctx, evt)
	if err != nil && s.logger != nil {
		s.logger.Error("webhook publish failed", "event", evt.Type, "event_id", evt.ID, "err", err)
	}
}
