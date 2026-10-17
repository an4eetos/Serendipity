// Package portstest provides fakes of the kernel ports for usecase tests.
package portstest

import "context"

type txKey struct{}

// TxManager runs units of work directly. It marks their context so that
// fake repositories can check that writes happen inside a transaction.
type TxManager struct {
	// Calls counts the outermost units of work.
	Calls int
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if InTx(ctx) {
		return fn(ctx)
	}
	m.Calls++
	return fn(context.WithValue(ctx, txKey{}, true))
}

// InTx reports whether ctx belongs to a unit of work.
func InTx(ctx context.Context) bool {
	in, _ := ctx.Value(txKey{}).(bool)
	return in
}
