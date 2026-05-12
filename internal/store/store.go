package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Config is the configuration for the database.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	Timezone string
	Schema   string
	LogSql   bool

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type Store struct {
	db     *sql.DB
	logSQL bool
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func New(cfg Config) (*Store, error) {
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("can't load location %s: %w", cfg.Timezone, err)
	}
	_ = location

	pgxConfig, err := pgx.ParseConfig(fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database,
	))
	if err != nil {
		return nil, fmt.Errorf("error parsing database config: %w", err)
	}
	pgxConfig.RuntimeParams["search_path"] = cfg.Schema
	pgxConfig.RuntimeParams["timezone"] = cfg.Timezone

	sqlDB, err := sql.Open("pgx", stdlib.RegisterConnConfig(pgxConfig))
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	c := Store{db: sqlDB, logSQL: cfg.LogSql}
	return &c, nil
}
