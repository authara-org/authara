package store

import (
	"context"
	"database/sql"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (s *Store) CurrentSchemaVersion(ctx context.Context) (string, error) {
	var version sql.NullString

	silentDB := s.db.Session(&gorm.Session{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	err := silentDB.WithContext(ctx).
		Raw(`
			SELECT id
			FROM public.schema_migrations
			ORDER BY applied_at DESC
			LIMIT 1
		`).
		Scan(&version).Error

	if err != nil {
		slog.Error(
			"failed to query schema version",
			"err", err,
		)
		return "", err
	}

	if !version.Valid {
		return "", nil
	}

	return version.String, nil
}
