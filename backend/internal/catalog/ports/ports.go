// Package ports declares what the catalog context needs from the outside.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
)

// Catalog searches an external book catalog.
type Catalog interface {
	Search(ctx context.Context, query string) ([]domain.SearchResult, error)
}

// WorkRepository stores works.
type WorkRepository interface {
	// Ensure returns the stored work with w's Open Library key, creating it
	// from w if there is none. Works without a key are always created.
	Ensure(ctx context.Context, w domain.Work, createdBy string) (domain.Work, error)
}
