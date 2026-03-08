package testutil

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
)

type TestDB struct {
	Store *store.Store
	Tx    *tx.Manager
}

func OpenTestDB(t *testing.T) *TestDB {
	t.Helper()

	cfg := store.Config{
		Host:     getenv("POSTGRESQL_HOST", "localhost"),
		Port:     getenvInt("POSTGRESQL_PORT", 5432),
		Database: getenv("POSTGRESQL_DATABASE", "authara_test"),
		Username: getenv("POSTGRESQL_USERNAME", "authara"),
		Password: getenv("POSTGRESQL_PASSWORD", "authara"),
		Schema:   getenv("POSTGRESQL_SCHEMA", "public"),
		Timezone: getenv("POSTGRESQL_TIMEZONE", "UTC"),
		LogSql:   getenvBool("POSTGRESQL_LOG_SQL", false),
	}

	st, err := store.New(cfg)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	return &TestDB{
		Store: st,
		Tx:    tx.New(st),
	}
}

func WithRollbackTx(t *testing.T, tdb *TestDB, fn func(ctx context.Context)) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	txCtx, cancelTx, err := startTxContext(tdb, ctx)
	if err != nil {
		t.Fatalf("start tx: %v", err)
	}
	defer cancelTx()

	fn(txCtx)

	_ = tdb.Tx.Rollback(txCtx)
}

func startTxContext(tdb *TestDB, parent context.Context) (context.Context, context.CancelFunc, error) {
	return tdb.Tx.Begin(parent)
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}

	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return def
	}
}
