package keyset

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type testItem struct {
	createdAt time.Time
	id        uuid.UUID
}

func TestPaginationEmptyFirstMiddleAndFinal(t *testing.T) {
	scope := uuid.New()
	base := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	items := []testItem{
		{base, uuid.MustParse("00000000-0000-0000-0000-000000000001")},
		{base, uuid.MustParse("00000000-0000-0000-0000-000000000002")},
		{base.Add(time.Second), uuid.MustParse("00000000-0000-0000-0000-000000000003")},
	}
	key := func(item testItem) (time.Time, uuid.UUID) { return item.createdAt, item.id }

	empty, err := Finish([]testItem{}, 2, "items", scope, key)
	if err != nil || len(empty.Items) != 0 || empty.NextCursor != "" {
		t.Fatalf("empty page = %+v, err = %v", empty, err)
	}
	first, err := Finish(items, 2, "items", scope, key)
	if err != nil || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %+v, err = %v", first, err)
	}
	boundary, limit, err := Decode(Options{Cursor: first.NextCursor, Limit: intPtr(2)}, "items", scope)
	if err != nil || limit != 2 || !boundary.CreatedAt.Equal(items[1].createdAt) || boundary.ID != items[1].id {
		t.Fatalf("middle boundary = %+v, limit = %d, err = %v", boundary, limit, err)
	}
	final, err := Finish(items[2:], 2, "items", scope, key)
	if err != nil || len(final.Items) != 1 || final.NextCursor != "" {
		t.Fatalf("final page = %+v, err = %v", final, err)
	}
}

func TestPaginationRejectsInvalidCursorScopeKindAndLimit(t *testing.T) {
	scope := uuid.New()
	item := testItem{time.Now().UTC(), uuid.New()}
	page, err := Finish([]testItem{item, {time.Now().UTC(), uuid.New()}}, 1, "items", scope, func(item testItem) (time.Time, uuid.UUID) {
		return item.createdAt, item.id
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []Options{
		{Cursor: "not-base64"},
		{Cursor: base64.RawURLEncoding.EncodeToString([]byte("{"))},
		{Cursor: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"kind":"items","extra":true}`))},
		{Cursor: base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"kind":"items","scope":"` + scope.String() + `","created_at":"` + item.createdAt.Format(time.RFC3339Nano) + `","id":"` + item.id.String() + `"}`))},
		{Cursor: page.NextCursor, Limit: intPtr(0)},
		{Cursor: page.NextCursor, Limit: intPtr(MaxLimit + 1)},
	} {
		if _, _, err := Decode(options, "items", scope); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("Decode(%+v) error = %v", options, err)
		}
	}
	if _, _, err := Decode(Options{Cursor: page.NextCursor}, "other", scope); !errors.Is(err, ErrInvalidPage) {
		t.Fatalf("wrong kind error = %v", err)
	}
	if _, _, err := Decode(Options{Cursor: page.NextCursor}, "items", uuid.New()); !errors.Is(err, ErrInvalidPage) {
		t.Fatalf("wrong scope error = %v", err)
	}
}

func intPtr(value int) *int { return &value }
