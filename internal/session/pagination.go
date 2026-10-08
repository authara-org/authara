package session

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/keyset"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const sessionsCursorKind = "account_sessions"

var ErrInvalidListPage = errors.New("invalid session list page")

type ListOptions = keyset.Options
type SessionPage = keyset.Page[domain.Session]

func (s *Service) ListUserSessionsPage(ctx context.Context, userID uuid.UUID, now time.Time, options ListOptions) (SessionPage, error) {
	boundary, limit, err := keyset.Decode(options, sessionsCursorKind, userID)
	if err != nil {
		return SessionPage{}, ErrInvalidListPage
	}
	var cursor *store.ListCursor
	if boundary != nil {
		cursor = &store.ListCursor{CreatedAt: boundary.CreatedAt, ID: boundary.ID}
	}
	items, err := s.store.ListActiveSessionsPageByUserID(ctx, userID, now, cursor, limit+1)
	if err != nil {
		return SessionPage{}, err
	}
	return keyset.Finish(items, limit, sessionsCursorKind, userID, func(item domain.Session) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	})
}
