package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/authara-org/authara/internal/logging"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	s.logDatabaseOperation(ctx, logging.OperationOther)
	return s.query(ctx).ExecContext(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	s.logDatabaseOperation(ctx, logging.OperationSnapshot)
	return s.query(ctx).QueryRowContext(ctx, query, args...)
}

func (s *Store) queryRows(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	s.logDatabaseOperation(ctx, logging.OperationSnapshot)
	return s.query(ctx).QueryContext(ctx, query, args...)
}

func (s *Store) logDatabaseOperation(ctx context.Context, operation logging.Operation) {
	if !s.logSQL || s.logger == nil {
		return
	}
	s.logger.Log(ctx, logging.LevelDebug, logging.EventInternalOperation,
		logging.ComponentField(logging.ComponentDatabase),
		logging.OperationField(operation),
		logging.OutcomeField(logging.OutcomeStarted),
	)
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
