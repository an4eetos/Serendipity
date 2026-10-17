package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// TxManager implements kernelports.TxManager on the DB's pool.
type TxManager struct {
	db *DB
}

func NewTxManager(db *DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := m.db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rolling back a committed transaction is a no-op. Deferring it also
	// covers panics, and must still reach the server if ctx was canceled.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return translate(tx.Commit(ctx))
}
