package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/session/roles"
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
	AccessPolicy     accesspolicy.EmailAccessPolicy
}

type Service struct {
	store            *store.Store
	tx               *tx.Manager
	webhookPublisher webhook.Publisher
	logger           *slog.Logger
	accessPolicy     accesspolicy.EmailAccessPolicy
}

func New(cfg Config) *Service {
	pub := cfg.WebhookPublisher
	if pub == nil {
		pub = webhook.NoopPublisher{}
	}
	access := cfg.AccessPolicy
	if access == nil {
		access = accesspolicy.NoopEmailAccessPolicy{}
	}

	return &Service{
		store:            cfg.Store,
		tx:               cfg.Tx,
		webhookPublisher: pub,
		logger:           cfg.Logger,
		accessPolicy:     access,
	}
}

func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Service) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type CurrentUser struct {
	User  domain.User
	Roles []roles.Role
}

func (s *Service) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*CurrentUser, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles, ok := httpctx.Roles(ctx)
	if !ok {
		return nil, ErrNoRolesInContext
	}

	cu := CurrentUser{
		User:  user,
		Roles: roles.List(),
	}

	return &cu, nil
}

func (s *Service) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	if err := s.store.DeleteUser(ctx, userID); err != nil {
		return err
	}

	s.publishBestEffort(ctx, webhook.NewUserDeleted(userID, time.Now()))

	return nil
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*domain.User, error) {
	if in.Username == "" {
		local := strings.SplitN(in.Email, "@", 2)[0]
		local = SanitizeUsername(local) // you can choose whether this lowercases or not
		if local == "" {
			local = "user"
		}

		suffix, err := SecureFiveDigits()
		if err != nil {
			return nil, err
		}

		// If you want generated usernames always lowercase:
		local = strings.ToLower(local)

		in.Username = fmt.Sprintf("%s-%05d", local, suffix)
	}

	allowed, err := s.accessPolicy.IsEmailAllowed(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrEmailNotAllowed
	}

	switch in.Provider {
	case domain.ProviderPassword:
		return s.loginWithPassword(ctx, in)

	case domain.ProviderGoogle:
		return s.loginWithExternalIdentity(ctx, in)

	default:
		return nil, ErrUnsupportedProvider
	}
}

func (s *Service) Signup(ctx context.Context, in SignupInput) (*domain.User, error) {
	if in.Username == "" {
		local := strings.SplitN(in.Email, "@", 2)[0]
		local = SanitizeUsername(local) // you can choose whether this lowercases or not
		if local == "" {
			local = "user"
		}

		suffix, err := SecureFiveDigits()
		if err != nil {
			return nil, err
		}

		// If you want generated usernames always lowercase:
		local = strings.ToLower(local)

		in.Username = fmt.Sprintf("%s-%05d", local, suffix)
	}

	allowed, err := s.accessPolicy.IsEmailAllowed(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrEmailNotAllowed
	}

	switch in.Provider {
	case domain.ProviderPassword:
		user, err := s.signupWithPassword(ctx, in)
		if err != nil {
			return nil, err
		}
		s.publishBestEffort(ctx, webhook.NewUserCreated(user.ID, time.Now()))
		return user, nil

	default:
		return nil, ErrUnsupportedProvider
	}
}

func (s *Service) signupWithPassword(ctx context.Context, in SignupInput) (*domain.User, error) {
	var user domain.User

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		user = domain.User{
			Email:    in.Email,
			Username: in.Username,
		}

		created, err := s.store.CreateUser(txCtx, user)
		if err != nil {
			if store.IsUniqueViolation(err, store.ConstraintUserEmail) {
				return ErrUserAlreadyExists
			}
			return err
		}
		user = created

		provider := domain.AuthProvider{
			UserID:         user.ID,
			Provider:       domain.ProviderPassword,
			PasswordHash:   &in.PasswordHash,
			ProviderUserID: nil,
		}

		_, err = s.store.CreateAuthProvider(txCtx, provider)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Service) loginWithPassword(ctx context.Context, in LoginInput) (*domain.User, error) {
	user, err := s.store.GetUserByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}

	authPovider, err := s.store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
	if err != nil {
		return nil, err
	}

	verified, err := Verify(in.Password, *authPovider.PasswordHash)
	if err != nil || !verified {
		return nil, err
	}

	return &user, nil
}

func (s *Service) loginWithExternalIdentity(ctx context.Context, in LoginInput) (*domain.User, error) {
	var user domain.User
	createdUser := false

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {

		providerRecord, err := s.store.GetAuthProviderByProviderAndProviderUserID(txCtx, in.Provider, in.OAuthID)
		if err == nil {
			// provider exists => just log in
			user, err = s.store.GetUserByID(txCtx, providerRecord.UserID)
			return err
		}

		if err != store.ErrorAuthProviderNotFound {
			return err
		}

		// provider does not exist
		emailExists, err := s.store.UserExistsByEmail(txCtx, in.Email)
		if err != nil {
			return err
		}

		if emailExists {
			// provider does not exist but user account with that email does
			return ErrAccountExistsMustLink
		}

		// provider does not exist and user account with that email also does not exist
		// => create user with that provider

		domainUser := domain.User{
			Email:    in.Email,
			Username: in.Username,
		}
		user, err = s.store.CreateUser(txCtx, domainUser)
		if err != nil {
			return err
		}
		createdUser = true

		domainProvider := domain.AuthProvider{
			UserID:         user.ID,
			Provider:       in.Provider,
			ProviderUserID: &in.OAuthID,
		}
		_, err = s.store.CreateAuthProvider(txCtx, domainProvider)

		return err
	})

	if err != nil {
		return nil, err
	}

	if createdUser {
		s.publishBestEffort(ctx, webhook.NewUserCreated(user.ID, time.Now()))
	}

	return &user, nil
}

func (s *Service) DisableUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()

	err := s.store.DisableUser(ctx, userID, now)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) ChangeUsername(ctx context.Context, userID uuid.UUID, username string) error {
	err := ValidateUsername(username)
	if err != nil {
		return err
	}

	err = s.store.UpdateUsername(ctx, userID, username)

	if err != nil {
		if store.IsUniqueViolation(err, store.ConstraintUserUsername) {
			return ErrUsernameTaken
		}
		return err
	}

	return nil
}

func (s *Service) publishBestEffort(ctx context.Context, evt webhook.Envelope) {
	err := s.webhookPublisher.Publish(ctx, evt)
	if err != nil && s.logger != nil {
		s.logger.Error("webhook publish failed", "event", evt.Type, "event_id", evt.ID, "err", err)
	}
}
