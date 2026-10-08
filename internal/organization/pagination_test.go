package organization

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type paginationItem struct {
	createdAt time.Time
	id        uuid.UUID
}

func TestOrganizationPaginationEmptyFirstMiddleAndFinalPages(t *testing.T) {
	scope := uuid.New()
	base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []paginationItem{
		{createdAt: base, id: uuid.MustParse("00000000-0000-0000-0000-000000000001")},
		{createdAt: base, id: uuid.MustParse("00000000-0000-0000-0000-000000000002")},
		{createdAt: base.Add(time.Second), id: uuid.MustParse("00000000-0000-0000-0000-000000000003")},
	}
	key := func(item paginationItem) (time.Time, uuid.UUID) { return item.createdAt, item.id }

	empty, err := finishPage([]paginationItem{}, 2, membersCursorKind, scope, key)
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty page = %+v, err = %v", empty, err)
	}

	first, err := finishPage(items, 2, membersCursorKind, scope, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %+v", first)
	}
	cursor, limit, err := decodeListOptions(ListOptions{Cursor: first.NextCursor, Limit: intPointer(2)}, membersCursorKind, scope)
	if err != nil {
		t.Fatal(err)
	}
	if limit != 2 || !cursor.CreatedAt.Equal(items[1].createdAt) || cursor.ID != items[1].id {
		t.Fatalf("decoded cursor = %+v, limit = %d", cursor, limit)
	}

	final, err := finishPage(items[2:], 2, membersCursorKind, scope, key)
	if err != nil || len(final.Items) != 1 || final.NextCursor != "" {
		t.Fatalf("final page = %+v, err = %v", final, err)
	}

	repeated, err := finishPage(items, 2, membersCursorKind, scope, key)
	if err != nil || repeated.NextCursor != first.NextCursor {
		t.Fatalf("cursor is not stable: first=%q repeated=%q err=%v", first.NextCursor, repeated.NextCursor, err)
	}
}

func TestOrganizationPaginationRejectsInvalidCursorAndLimit(t *testing.T) {
	scope := uuid.New()
	base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	validPage, err := finishPage([]paginationItem{{base, uuid.New()}, {base.Add(time.Second), uuid.New()}}, 1, membersCursorKind, scope, func(item paginationItem) (time.Time, uuid.UUID) {
		return item.createdAt, item.id
	})
	if err != nil {
		t.Fatal(err)
	}
	valid := validPage.NextCursor

	for _, tc := range []struct {
		name    string
		options ListOptions
		kind    string
		scope   uuid.UUID
	}{
		{name: "zero limit", options: ListOptions{Limit: intPointer(0)}, kind: membersCursorKind, scope: scope},
		{name: "negative limit", options: ListOptions{Limit: intPointer(-1)}, kind: membersCursorKind, scope: scope},
		{name: "above maximum", options: ListOptions{Limit: intPointer(MaxListLimit + 1)}, kind: membersCursorKind, scope: scope},
		{name: "malformed base64", options: ListOptions{Cursor: "%%%"}, kind: membersCursorKind, scope: scope},
		{name: "wrong collection", options: ListOptions{Cursor: valid}, kind: invitationsCursorKind, scope: scope},
		{name: "wrong scope", options: ListOptions{Cursor: valid}, kind: membersCursorKind, scope: uuid.New()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := decodeListOptions(tc.options, tc.kind, tc.scope); !errors.Is(err, ErrInvalidListPage) {
				t.Fatalf("error = %v, want ErrInvalidListPage", err)
			}
		})
	}

	_, defaultLimit, err := decodeListOptions(ListOptions{}, membersCursorKind, scope)
	if err != nil || defaultLimit != DefaultListLimit {
		t.Fatalf("default limit = %d, err = %v", defaultLimit, err)
	}
}

func intPointer(value int) *int { return &value }
