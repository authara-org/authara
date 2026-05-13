package bootstrap

import (
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
)

func NewStore(cfg *config.Config) (*store.Store, error) {
	return store.New(store.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		Username:        cfg.DB.Username,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		Schema:          cfg.DB.Schema,
		Timezone:        cfg.DB.Timezone,
		LogSql:          cfg.DB.LogSQL,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	})
}
