package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/schema"
)

type App struct {
	Config   *config.Config
	Logger   *slog.Logger
	Store    *store.Store
	Cache    cache.Cache
	Services Services
}

func NewApp() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger, err := NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	st, err := NewStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if err := CheckSchemaVersion(st, schema.RequiredSchemaVersion); err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("schema version check: %w", err)
	}

	ca, err := NewCache(cfg)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("connect cache: %w", err)
	}

	configureRuntime(cfg)

	a := &App{
		Config: cfg,
		Logger: logger,
		Store:  st,
		Cache:  ca,
	}
	services, err := NewServices(a)
	if err != nil {
		_ = ca.Close()
		_ = st.Close()
		return nil, err
	}
	a.Services = services
	return a, nil
}

func (a *App) Close() error {
	var errs []error

	if a.Cache != nil {
		if err := a.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cache: %w", err))
		}
	}
	if a.Store != nil {
		if err := a.Store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close store: %w", err))
		}
	}

	return errors.Join(errs...)
}
