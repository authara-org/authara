package store

import "context"

func (s *Store) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	var allowed bool
	return allowed, nil
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
