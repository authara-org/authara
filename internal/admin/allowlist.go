package admin

import (
	"context"
	"errors"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

func (s *Service) ListAllowedEmails(ctx context.Context, query string, page Page) (AllowedEmailPage, error) {
	if err := s.requireAllowlistEnabled(); err != nil {
		return AllowedEmailPage{}, err
	}

	page = normalizePage(page, 25)
	query = normalizeEmail(query)
	if query != "" && len(query) < 3 {
		return AllowedEmailPage{
			Query:   query,
			Page:    1,
			Size:    page.Size,
			Message: "Type at least 3 characters to search, or clear the field to show all emails.",
		}, nil
	}

	total, err := s.store.CountAllowedEmails(ctx, query)
	if err != nil {
		return AllowedEmailPage{}, err
	}
	emails, err := s.store.ListAllowedEmailsPage(ctx, query, page.Size, (page.Page-1)*page.Size)
	if err != nil {
		return AllowedEmailPage{}, err
	}
	return AllowedEmailPage{
		Emails: emails,
		Query:  query,
		Page:   page.Page,
		Size:   page.Size,
		Total:  total,
	}, nil
}

func (s *Service) AddAllowedEmail(ctx context.Context, actor Actor, email string, meta RequestMeta) error {
	if err := s.requireAllowlistEnabled(); err != nil {
		return err
	}

	email = normalizeEmail(email)
	if email == "" {
		return ErrInvalidEmail
	}

	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateAllowedEmail(txCtx, domain.AllowedEmail{Email: email}); err != nil {
			if errors.Is(err, store.ErrAllowedEmailAlreadyExists) {
				return ErrAllowedEmailAlreadyAdded
			}
			return err
		}
		return s.audit(txCtx, actor, ActionAllowlistEmailAdded, nil, email, map[string]any{}, meta)
	})
}

func (s *Service) RemoveAllowedEmail(ctx context.Context, actor Actor, allowedEmailID uuid.UUID, meta RequestMeta) error {
	if err := s.requireAllowlistEnabled(); err != nil {
		return err
	}

	return s.tx.WithTransaction(ctx, func(txCtx context.Context) error {
		allowedEmail, err := s.store.DeleteAllowedEmailByID(txCtx, allowedEmailID)
		if err != nil {
			return err
		}
		return s.audit(txCtx, actor, ActionAllowlistEmailRemoved, nil, allowedEmail.Email, map[string]any{
			"allowed_email_id": allowedEmail.ID.String(),
		}, meta)
	})
}

func (s *Service) requireAllowlistEnabled() error {
	if !s.allowlistEnabled {
		return ErrAllowlistDisabled
	}
	return nil
}
