package organization

import (
	"errors"
	"time"

	"github.com/authara-org/authara/internal/keyset"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const (
	DefaultListLimit = keyset.DefaultLimit
	MaxListLimit     = keyset.MaxLimit

	membersCursorKind       = "organization_members"
	membershipsCursorKind   = "user_memberships"
	organizationsCursorKind = "user_organizations"
	invitationsCursorKind   = "organization_invitations"
)

var ErrInvalidListPage = errors.New("invalid organization list page")

type ListOptions = keyset.Options

type Page[T any] struct {
	Items      []T
	NextCursor string
}

func decodeListOptions(options ListOptions, kind string, scope uuid.UUID) (*store.ListCursor, int, error) {
	boundary, limit, err := keyset.Decode(options, kind, scope)
	if err != nil {
		return nil, 0, ErrInvalidListPage
	}
	if boundary == nil {
		return nil, limit, nil
	}
	return &store.ListCursor{CreatedAt: boundary.CreatedAt, ID: boundary.ID}, limit, nil
}

func finishPage[T any](items []T, limit int, kind string, scope uuid.UUID, key func(T) (time.Time, uuid.UUID)) (Page[T], error) {
	page, err := keyset.Finish(items, limit, kind, scope, key)
	if err != nil {
		return Page[T]{}, err
	}
	return Page[T]{Items: page.Items, NextCursor: page.NextCursor}, nil
}
