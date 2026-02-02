package store

import (
	"context"
	"errors"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/domain"
	"github.com/alexlup06-authgate/authgate/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func toDomainUser(m model.User) domain.User {
	return domain.User{
		ID:        *m.ID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
		Username:  m.Username,
		Email:     m.Email,
	}
}

func toModelUser(d domain.User) model.User {
	return model.User{
		ID:       nil,
		Username: d.Username,
		Email:    d.Email,
	}
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	m := toModelUser(user)

	db := s.dbFromContext(ctx)

	err := db.
		Create(&m).
		Error

	if err != nil {
		return domain.User{}, err
	}

	return toDomainUser(m), nil
}

func (s *Store) GetUserByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	var m model.User

	err := s.dbFromContext(ctx).
		Where("id = ?", userID.String()).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return toDomainUser(m), nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var m model.User

	err := s.dbFromContext(ctx).
		Where("email = ?", email).
		First(&m).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return toDomainUser(m), nil
}

func (s *Store) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64

	err := s.dbFromContext(ctx).
		Model(&model.User{}).
		Where("email = ?", email).
		Count(&count).
		Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *Store) IsUserAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool

	err := s.dbFromContext(ctx).
		Raw(`
			SELECT EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN roles r ON r.ID = ur.role_id
				WHERE ur.user_id = ? AND r.name = 'admin'
			)
		`, userID).
		Scan(&exists).
		Error

	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *Store) DisableUser(ctx context.Context, userID uuid.UUID, disabledAt time.Time) error {
	return s.dbFromContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("disabled_at", disabledAt).
		Error
}

func (s *Store) IsUserDisabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool

	err := s.dbFromContext(ctx).
		Model(&model.User{}).
		Select("count(1) > 0").
		Where("id = ? AND disabled_at IS NOT NULL", userID).
		Find(&exists).
		Error

	return exists, err
}
