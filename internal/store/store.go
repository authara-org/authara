package store

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
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
	db *gorm.DB
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func New(cfg Config) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s TimeZone=%s search_path=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, cfg.Timezone, cfg.Schema,
	)

	var lg gormlogger.Interface
	if cfg.LogSql {
		lg = gormlogger.Default.LogMode(gormlogger.Info)
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("can't load location %s: %w", cfg.Timezone, err)
	}

	gormConfig := gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().In(location)
		},
		Logger:                 lg,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   cfg.Schema + ".",
			SingularTable: false,
		},
	}

	gormDB, err := gorm.Open(postgres.Open(dsn), &gormConfig)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	c := Store{db: gormDB}
	return &c, nil
}
