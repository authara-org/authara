package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.logSQL {
		slog.DebugContext(ctx, "sql exec", "query", query)
	}
	return s.query(ctx).ExecContext(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if s.logSQL {
		slog.DebugContext(ctx, "sql query row", "query", query)
	}
	return s.query(ctx).QueryRowContext(ctx, query, args...)
}

func (s *Store) queryRows(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.logSQL {
		slog.DebugContext(ctx, "sql query rows", "query", query)
	}
	return s.query(ctx).QueryContext(ctx, query, args...)
}

func mapNoRows(err error, mapped error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return mapped
	}
	return err
}

func nullableJSONBytes(b []byte) any {
	if b == nil {
		return nil
	}
	return string(b)
}
