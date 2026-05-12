package store

import (
	"context"
	"database/sql"
	"log/slog"
)

func (s *Store) CurrentSchemaVersion(ctx context.Context) (int, error) {
	var version sql.NullInt32

	err := s.db.QueryRowContext(ctx, `SELECT max(version) FROM public.authara_schema_version`).Scan(&version)

	if err != nil {
		slog.Error(
			"failed to query schema version",
			"err", err,
		)
		return 0, err
	}

	if !version.Valid {
		return 0, nil
	}

	return int(version.Int32), nil
}
