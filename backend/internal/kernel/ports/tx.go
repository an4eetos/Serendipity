// Package ports holds the ports every bounded context shares. Bounded
// contexts import it as "kernelports".
package ports

import "context"

// TxManager runs a unit of work in one database transaction.
type TxManager interface {
	// WithinTx runs fn in a transaction that commits when fn returns nil and
	// rolls back otherwise. Calls nested inside fn join the outer
	// transaction.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
