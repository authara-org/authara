package auth

import (
	"context"

	"github.com/alexlup06/authgate/internal/domain"
	"github.com/alexlup06/authgate/internal/store"
	"github.com/alexlup06/authgate/internal/store/tx"
)

type Config struct {
	Store *store.Store
	Tx    *tx.Manager
}

type Service struct {
	store *store.Store
	tx    *tx.Manager
}

func New(cfg Config) *Service {
	return &Service{
		store: cfg.Store,
		tx:    cfg.Tx,
	}
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*domain.User, error) {
	switch in.Provider {
	case ProviderPassword:
		return s.loginWithPassword(ctx, in)

	case ProviderGoogle, ProviderApple:
		return s.loginWithExternalIdentity(ctx, in)

	default:
		return nil, ErrUnsupportedProvider
	}
}

func (s *Service) Signup(ctx context.Context, in SignupInput) (*domain.User, error) {
	switch in.Provider {
	case ProviderPassword:
		return s.signupWithPassword(ctx, in)

	case ProviderGoogle, ProviderApple:
		return s.signupWithExternalIdentity(ctx, in)

	default:
		return nil, ErrUnsupportedProvider
	}
}

func (s *Service) signupWithPassword(ctx context.Context, in SignupInput) (*domain.User, error) {
	var user domain.User

	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		exists, err := s.store.UserExistsByEmail(txCtx, in.Email)
		if err != nil {
			return err
		}
		if exists {
			return ErrUserAlreadyExists
		}

		hash, err := Hash(in.Password)
		if err != nil {
			return err
		}

		user = domain.User{
			Email: in.Email,
		}

		created, err := s.store.CreateUser(txCtx, user)
		if err != nil {
			return err
		}
		user = created

		provider := domain.AuthProvider{
			UserID:         *user.ID,
			Provider:       domain.ProviderPassword,
			PasswordHash:   &hash,
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

func (s *Service) signupWithExternalIdentity(ctx context.Context, in SignupInput) (*domain.User, error) {
	return &domain.User{}, nil
}

func (s *Service) loginWithPassword(ctx context.Context, in LoginInput) (*domain.User, error) {
	user, err := s.store.GetUserByEmail(ctx, in.Email)
	if err != nil {
		return &domain.User{}, err
	}

	authPovider, err := s.store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, *user.ID)
	if err != nil {
		return &domain.User{}, err
	}

	verified, err := Verify(in.Password, *authPovider.PasswordHash)
	if err != nil || !verified {
		return &domain.User{}, err
	}

	return &user, nil
}

func (s *Service) loginWithExternalIdentity(ctx context.Context, in LoginInput) (*domain.User, error) {
	return &domain.User{}, nil
}

//
// // Login validates credentials and returns the user.
// func (s *Service) Login(
// 	ctx context.Context,
// 	email string,
// 	password string,
// ) (*domain.User, error) {
// 	var (
// 		user     domain.User
// 		provider domain.AuthProvider
// 	)
//
// 	err := s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
// 		var err error
//
// 		user, err = s.store.GetUserByEmail(txCtx, email)
// 		if err != nil {
// 			return ErrInvalidCredentials
// 		}
//
// 		provider, err = s.store.GetPasswordProviderByUserID(txCtx, user.ID)
// 		if err != nil {
// 			return ErrInvalidCredentials
// 		}
//
// 		if !verifyPassword(provider.PasswordHash, password) {
// 			return ErrInvalidCredentials
// 		}
//
// 		return nil
// 	})
//
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return &user, nil
// }
