package store

import (
	"context"

	"gorm.io/gorm"
)

type contextKey string

const (
	TxKey contextKey = "store.tx.state"
	DbKey contextKey = "store.tx.db"
)

func (s *Store) dbFromContext(ctx context.Context) *gorm.DB {
	if txDB, ok := ctx.Value(DbKey).(*gorm.DB); ok {
		return txDB
	}
	return s.db
}

func (s *Store) query(ctx context.Context) *gorm.DB {
	return s.dbFromContext(ctx).WithContext(ctx)
}
