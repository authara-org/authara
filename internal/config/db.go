package config

import (
	"fmt"
	"time"
)

type DB struct {
	Host     string `env:"POSTGRESQL_HOST,required"`
	Port     int    `env:"POSTGRESQL_PORT,required"`
	Username string `env:"POSTGRESQL_USERNAME,required"`
	Password string `env:"POSTGRESQL_PASSWORD,required"`
	Database string `env:"POSTGRESQL_DATABASE,required"`
	Schema   string `env:"POSTGRESQL_SCHEMA,default=authara"`
	Timezone string `env:"POSTGRESQL_TIMEZONE,default=UTC"`
	LogSQL   bool   `env:"POSTGRESQL_LOG_SQL,default=false"`

	MaxOpenConns    int           `env:"AUTHARA_DB_MAX_OPEN_CONNS,default=40"`
	MaxIdleConns    int           `env:"AUTHARA_DB_MAX_IDLE_CONNS,default=20"`
	ConnMaxLifetime time.Duration `env:"AUTHARA_DB_CONN_MAX_LIFETIME,default=30m"`
	ConnMaxIdleTime time.Duration `env:"AUTHARA_DB_CONN_MAX_IDLE_TIME,default=5m"`
}

func (db DB) validate() error {
	if db.Host == "" {
		return fmt.Errorf("POSTGRESQL_HOST must not be empty")
	}
	if db.Port <= 0 || db.Port > 65535 {
		return fmt.Errorf("invalid POSTGRESQL_PORT %d", db.Port)
	}
	if db.Database == "" {
		return fmt.Errorf("POSTGRESQL_DATABASE must not be empty")
	}
	if db.Schema == "" {
		return fmt.Errorf("POSTGRESQL_SCHEMA must not be empty")
	}
	if db.Timezone == "" {
		return fmt.Errorf("POSTGRESQL_TIMEZONE must not be empty")
	}

	// --- connection pool validation ---
	if db.MaxOpenConns <= 0 {
		return fmt.Errorf("POSTGRESQL_MAX_OPEN_CONNS must be > 0")
	}
	if db.MaxIdleConns < 0 {
		return fmt.Errorf("POSTGRESQL_MAX_IDLE_CONNS must be >= 0")
	}
	if db.MaxIdleConns > db.MaxOpenConns {
		return fmt.Errorf("POSTGRESQL_MAX_IDLE_CONNS cannot be greater than POSTGRESQL_MAX_OPEN_CONNS")
	}

	if db.ConnMaxLifetime < 0 {
		return fmt.Errorf("POSTGRESQL_CONN_MAX_LIFETIME must be >= 0")
	}
	if db.ConnMaxIdleTime < 0 {
		return fmt.Errorf("POSTGRESQL_CONN_MAX_IDLE_TIME must be >= 0")
	}

	return nil
}
