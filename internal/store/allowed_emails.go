package store

import (
	"context"

	"github.com/authara-org/authara/internal/store/model"
)

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

func (s *Store) CreateAllowedEmail(ctx context.Context, email string) error {
	return nil
}

func (s *Store) DeleteAllowedEmail(ctx context.Context, email string) error {
	return nil
}

// func (s *Store) ListAllowedEmails(ctx context.Context) ([]domain.AllowedEmail, error) {
//
// }
