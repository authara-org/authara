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

		UserID:   m.UserID,
		Provider: domain.Provider(m.Provider),

		ProviderUserID: m.ProviderUserID,
		PasswordHash:   m.PasswordHash,
	}
}

func toModelAuthProvider(d domain.AuthProvider) model.AuthProvider {
	return model.AuthProvider{
		ID:       nil,
		UserID:   d.UserID,
		Provider: string(d.Provider),

		ProviderUserID: d.ProviderUserID,
		PasswordHash:   d.PasswordHash,
	}
}

func (s *Store) ListAuthProvidersByUserID(ctx context.Context, userID uuid.UUID) ([]domain.AuthProvider, error) {
	var rows []model.AuthProvider

	err := s.query(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&rows).
		Error
	if err != nil {
		return nil, err
	}

	out := make([]domain.AuthProvider, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainAuthProvider(row))
	}

	return out, nil
}

func (s *Store) CreateAuthProvider(ctx context.Context, provider domain.AuthProvider) (domain.AuthProvider, error) {
	m := toModelAuthProvider(provider)

	err := s.query(ctx).
		Create(&m).
		Error
	if err != nil {
		return domain.AuthProvider{}, err
	}

	return toDomainAuthProvider(m), nil
}

func (s *Store) GetAuthProviderByMethodAndUserID(ctx context.Context, provider domain.Provider, userID uuid.UUID) (domain.AuthProvider, error) {
	var m model.AuthProvider

	err := s.query(ctx).
		Where("user_id = ? AND provider = ?", userID, string(provider)).
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

	err := s.query(ctx).
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

func (s *Store) DeleteAuthProviderByMethodAndUserID(ctx context.Context, provider domain.Provider, userID uuid.UUID) error {
	res := s.query(ctx).
		Where("user_id = ? AND provider = ?", userID, string(provider)).
		Delete(&model.AuthProvider{})

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrorAuthProviderNotFound
	}

	return nil
}

func (s *Store) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	res := s.query(ctx).
		Model(&model.AuthProvider{}).
		Where("user_id = ? AND provider = ?", userID, "password").
		Update("password_hash", passwordHash)

	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrorAuthProviderNotFound
	}

	return nil
}
