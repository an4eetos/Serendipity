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
	// ListPublic lists the reader's newest public quotes.
	ListPublic(ctx context.Context, userID string, limit int) ([]domain.Quote, error)
	Update(ctx context.Context, userID, id string, p domain.Patch) (domain.Quote, error)
	Delete(ctx context.Context, userID, id string) error
}
