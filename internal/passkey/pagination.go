package passkey

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/keyset"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const passkeysCursorKind = "account_passkeys"

var ErrInvalidListPage = errors.New("invalid passkey list page")

type ListOptions = keyset.Options
type PasskeyPage = keyset.Page[domain.Passkey]

func (s *Service) ListUserPasskeysPage(ctx context.Context, userID uuid.UUID, options ListOptions) (PasskeyPage, error) {
	boundary, limit, err := keyset.Decode(options, passkeysCursorKind, userID)
	if err != nil {
		return PasskeyPage{}, ErrInvalidListPage
	}
	var cursor *store.ListCursor
	if boundary != nil {
		cursor = &store.ListCursor{CreatedAt: boundary.CreatedAt, ID: boundary.ID}
	}
	items, err := s.store.ListPasskeysPageByUserID(ctx, userID, cursor, limit+1)
	if err != nil {
		return PasskeyPage{}, err
	}
	return keyset.Finish(items, limit, passkeysCursorKind, userID, func(item domain.Passkey) (time.Time, uuid.UUID) {
		return item.CreatedAt, item.ID
	})
}
