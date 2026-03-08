package store

import (
	"context"
	"errors"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func toDomainAuthProvider(m model.AuthProvider) domain.AuthProvider {
	return domain.AuthProvider{
		ID:        *m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		UserID:   uuid.MustParse(m.UserID),
		Provider: domain.Provider(m.Provider),

		ProviderUserID: m.ProviderUserID,
		PasswordHash:   m.PasswordHash,
	}
}

func toModelAuthProvider(d domain.AuthProvider) model.AuthProvider {
	return model.AuthProvider{
		ID:       nil,
		UserID:   d.UserID.String(),
		Provider: string(d.Provider),

		ProviderUserID: d.ProviderUserID,
		PasswordHash:   d.PasswordHash,
	}
}

func (s *Store) CreateAuthProvider(ctx context.Context, provider domain.AuthProvider) (domain.AuthProvider, error) {
	m := toModelAuthProvider(provider)

	err := s.dbFromContext(ctx).
		Create(&m).
		Error

	if err != nil {
		return domain.AuthProvider{}, err
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) GetAuthProviderByMethodAndUserID(ctx context.Context, provider domain.Provider, userID uuid.UUID) (domain.AuthProvider, error) {
	var m model.AuthProvider

	err := s.dbFromContext(ctx).
		Where("user_id = ? AND provider = ?", userID.String(), string(provider)).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AuthProvider{}, ErrorAuthProviderNotFound
		}
		return domain.AuthProvider{}, err
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) GetAuthProviderByProviderAndProviderUserID(ctx context.Context, provider domain.Provider, providerUserID string) (domain.AuthProvider, error) {
	var m model.AuthProvider

	err := s.dbFromContext(ctx).
		Where("provider_user_id = ? AND provider = ?", providerUserID, string(provider)).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.AuthProvider{}, ErrorAuthProviderNotFound
		}
		return domain.AuthProvider{}, err
	}

	return toDomainAuthProvider(m), nil
}
