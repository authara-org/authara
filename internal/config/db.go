package config

import "fmt"

type DB struct {
	Host     string `env:"POSTGRESQL_HOST,required"`
	Port     int    `env:"POSTGRESQL_PORT,required"`
	Username string `env:"POSTGRESQL_USERNAME,required"`
	Password string `env:"POSTGRESQL_PASSWORD,required"`
	Database string `env:"POSTGRESQL_DATABASE,required"`
	Schema   string `env:"POSTGRESQL_SCHEMA,default=authgate"`
	Timezone string `env:"POSTGRESQL_TIMEZONE,default=UTC"`
	LogSQL   bool   `env:"POSTGRESQL_LOG_SQL,default=false"`
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
	return nil
}
