package tx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alexlup06-authgate/authgate/internal/store"
	"gorm.io/gorm"
)

type Manager struct {
	store *store.Store
}

func New(store *store.Store) *Manager {
	return &Manager{store: store}
}

type transactionState struct {
	committed  bool
	rolledBack bool
}

// ---------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------

func (m *Manager) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	txCtx, cancel, owned, err := m.withCancel(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	if err := fn(txCtx); err != nil {
		if owned && !isFinished(txCtx) {
			_ = m.Rollback(txCtx)
		}
		return err
	}

	if owned && !isFinished(txCtx) {
		return m.Commit(txCtx)
	}

	return nil
}

func (m *Manager) Commit(ctx context.Context) error {
	db, err := getDB(ctx)
	if err != nil {
		return err
	}

	if err := db.Commit().Error; err != nil {
		return err
	}

	setCommitted(ctx)
	return nil
}

func (m *Manager) Rollback(ctx context.Context) error {
	if isCommitted(ctx) {
		return errors.New("transaction already committed")
	}

	db, err := getDB(ctx)
	if err != nil {
		return err
	}

	if err := db.Rollback().Error; err != nil {
		return err
	}

	setRolledBack(ctx)
	return nil
}

// Begin starts a transaction if none exists yet.
// If the parent context already contains a transaction, it is reused.
func (m *Manager) Begin(parent context.Context) (context.Context, context.CancelFunc, error) {
	txCtx, cancel, _, err := m.withCancel(parent)
	return txCtx, cancel, err
}

// ---------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------

func (m *Manager) withCancel(parent context.Context) (context.Context, context.CancelFunc, bool, error) {
	// Reuse existing transaction if already present in context.
	if existing, ok := parent.Value(store.DbKey).(*gorm.DB); ok && existing != nil {
		// ensure tx state exists
		if _, ok := parent.Value(store.TxKey).(*transactionState); ok {
			return parent, func() {}, false, nil
		}
		ctx := context.WithValue(parent, store.TxKey, &transactionState{})
		return ctx, func() {}, false, nil
	}

	ctx, cancel := context.WithCancel(parent)
	txCtx, err := m.withContext(ctx)
	if err != nil {
		cancel()
		return nil, func() {}, false, err
	}
	return txCtx, cancel, true, nil
}

func (m *Manager) withContext(parent context.Context) (context.Context, error) {
	session := m.store.DB().
		WithContext(parent).
		Begin(&sql.TxOptions{
			Isolation: sql.LevelReadCommitted,
		})

	if session.Error != nil {
		return nil, session.Error
	}

	ctx := context.WithValue(parent, store.DbKey, session)
	ctx = context.WithValue(ctx, store.TxKey, &transactionState{})

	go m.cleanup(ctx)

	return ctx, nil
}

func (m *Manager) cleanup(ctx context.Context) {
	<-ctx.Done()
	if !isFinished(ctx) {
		_ = m.Rollback(ctx)
	}
}

// ---------------------------------------------------------------------
// Context utilities
// ---------------------------------------------------------------------

func getDB(ctx context.Context) (*gorm.DB, error) {
	db, ok := ctx.Value(store.DbKey).(*gorm.DB)
	if !ok {
		return nil, fmt.Errorf("no transaction in context")
	}
	return db, nil
}

func isCommitted(ctx context.Context) bool {
	state, ok := ctx.Value(store.TxKey).(*transactionState)
	return ok && state.committed
}

func isFinished(ctx context.Context) bool {
	state, ok := ctx.Value(store.TxKey).(*transactionState)
	return ok && (state.committed || state.rolledBack)
}

func setCommitted(ctx context.Context) {
	if state, ok := ctx.Value(store.TxKey).(*transactionState); ok {
		state.committed = true
	}
}

func setRolledBack(ctx context.Context) {
	if state, ok := ctx.Value(store.TxKey).(*transactionState); ok {
		state.rolledBack = true
	}
}
