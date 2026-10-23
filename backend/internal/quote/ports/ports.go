// Package ports declares what the quote context needs from the outside.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// QuoteRepository stores readers' quotes. Every method is scoped to the
// reader's own quotes.
type QuoteRepository interface {
	// Create stores a quote, or returns the reader's existing quote with the
	// same text from the same work.
	Create(ctx context.Context, userID string, d domain.Draft) (domain.Quote, error)
	// Import stores a quote unless the reader already has the same text from
	// the same work, and reports whether it did.
	Import(ctx context.Context, userID string, d domain.Draft) (bool, error)
	Get(ctx context.Context, userID, id string) (domain.Quote, error)
	// List lists the reader's quotes in reading order, of one work if workID
	// is not empty.
	List(ctx context.Context, userID, workID string) ([]domain.Quote, error)
	// ListPublic lists the reader's newest listed quotes (public, not removed).
	ListPublic(ctx context.Context, userID string, limit int) ([]domain.Quote, error)
	// Update applies p, including PassageHidden when set.
	Update(ctx context.Context, userID, id string, p domain.Patch) (domain.Quote, error)
	Delete(ctx context.Context, userID, id string) error
}

// CoverageRepository measures coverage. Its methods must run inside the
// TxManager.
type CoverageRepository interface {
	// SetCap makes wordCap the default cap the coverage trigger applies for
	// the rest of the transaction (transaction-local setting). Call it before
	// any quote write that can move a quote into coverage.
	SetCap(ctx context.Context, wordCap domain.CoverageCap) error
	// LockWork holds the work's lock until the transaction ends.
	LockWork(ctx context.Context, workID string) error
	// Coverage measures the work's coverage without the quote quoteID.
	Coverage(ctx context.Context, workID, quoteID string) (domain.Coverage, error)
}

// QuoteModeration is how moderators act on any reader's quotes; it is not
// scoped to a reader.
type QuoteModeration interface {
	// ListAny returns the quotes with these ids, removed ones included;
	// missing ids are left out.
	ListAny(ctx context.Context, ids []string) ([]domain.Quote, error)
	// Remove sets removed_at/removed_reason. It returns kernel.ErrNotFound
	// unless the quote exists and isn't removed.
	Remove(ctx context.Context, id, reason string) error
	// Restore clears them, which moves the quote back into coverage. It must
	// run inside the TxManager. It first sets wordCap as the transaction's
	// default cap (same as CoverageRepository.SetCap), then updates, and the
	// trigger re-checks the cap. It returns kernel.ErrNotFound unless the
	// quote exists and is removed.
	Restore(ctx context.Context, id string, wordCap domain.CoverageCap) error
}
