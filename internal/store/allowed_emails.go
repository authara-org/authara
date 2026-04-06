package store

import (
	"context"
	"errors"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"gorm.io/gorm"
)

func toDomainAllowedEmail(m model.AllowedEmail) domain.AllowedEmail {
	return domain.AllowedEmail{
		ID:    *m.ID,
		Email: m.Email,
	}
}

func toModelAllowedEmail(d domain.AllowedEmail) model.AllowedEmail {
	return model.AllowedEmail{
		ID:    nil,
		Email: d.Email,
	}
}

func (s *Store) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	var count int64

	err := s.dbFromContext(ctx).
		Model(&model.AllowedEmail{}).
		Where("email = ?", email).
		Count(&count).
		Error

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) CreateAllowedEmail(ctx context.Context, allowedEmail domain.AllowedEmail) error {
	m := toModelAllowedEmail(allowedEmail)

	return s.dbFromContext(ctx).Create(&m).Error
}

func (s *Store) DeleteAllowedEmail(ctx context.Context, email string) error {
	result := s.dbFromContext(ctx).
		Where("email = ?", email).
		Delete(&model.AllowedEmail{})

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (s *Store) ListAllowedEmails(ctx context.Context) ([]domain.AllowedEmail, error) {
	var rows []model.AllowedEmail

	err := s.dbFromContext(ctx).
		Order("email ASC").
		Find(&rows).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []domain.AllowedEmail{}, nil
		}
		return nil, err
	}

	out := make([]domain.AllowedEmail, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainAllowedEmail(row))
	}

	return out, nil
}
