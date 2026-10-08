package keyset

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

var ErrInvalidPage = errors.New("invalid list page")

type Options struct {
	Cursor string
	Limit  *int
}

type Boundary struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type Page[T any] struct {
	Items      []T
	NextCursor string
}

type cursor struct {
	Version   int       `json:"v"`
	Kind      string    `json:"kind"`
	Scope     uuid.UUID `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

func Decode(options Options, kind string, scope uuid.UUID) (*Boundary, int, error) {
	limit := DefaultLimit
	if options.Limit != nil {
		limit = *options.Limit
	}
	if limit < 1 || limit > MaxLimit {
		return nil, 0, ErrInvalidPage
	}
	if options.Cursor == "" {
		return nil, limit, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(options.Cursor)
	if err != nil {
		return nil, 0, ErrInvalidPage
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value cursor
	decodeErr := decoder.Decode(&value)
	var trailing any
	trailingErr := decoder.Decode(&trailing)
	if decodeErr != nil || trailingErr != io.EOF || value.Version != 1 || value.Kind != kind || value.Scope != scope || value.CreatedAt.IsZero() || value.ID == uuid.Nil {
		return nil, 0, ErrInvalidPage
	}
	return &Boundary{CreatedAt: value.CreatedAt, ID: value.ID}, limit, nil
}

func Finish[T any](items []T, limit int, kind string, scope uuid.UUID, key func(T) (time.Time, uuid.UUID)) (Page[T], error) {
	page := Page[T]{Items: items}
	if len(items) <= limit {
		return page, nil
	}
	page.Items = items[:limit]
	createdAt, id := key(page.Items[len(page.Items)-1])
	raw, err := json.Marshal(cursor{Version: 1, Kind: kind, Scope: scope, CreatedAt: createdAt.UTC(), ID: id})
	if err != nil {
		return Page[T]{}, err
	}
	page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	return page, nil
}
