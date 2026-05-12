package store

import (
	"context"
	"database/sql"
)

type contextKey string

const (
	TxKey contextKey = "store.tx.state"
	DbKey contextKey = "store.tx.db"
)

type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) dbFromContext(ctx context.Context) queryer {
	if txDB, ok := ctx.Value(DbKey).(*sql.Tx); ok {
		return txDB
	}
	return s.db
}

func (s *Store) query(ctx context.Context) queryer {
	return s.dbFromContext(ctx)
}
