package store

import (
	"context"
	"database/sql"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (s *Store) CurrentSchemaVersion(ctx context.Context) (int, error) {
	var version sql.NullInt32

	silentDB := s.db.Session(&gorm.Session{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	err := silentDB.WithContext(ctx).
		Raw(`SELECT version FROM public.authara_schema_version LIMIT 1`).
		Scan(&version).Error

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
