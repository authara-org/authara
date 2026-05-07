package store

import "context"

func (s *Store) Ping(ctx context.Context) error {
	db := s.query(ctx)

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.PingContext(ctx)
}
