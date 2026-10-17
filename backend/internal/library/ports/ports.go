// Package ports declares what the library context needs from the outside.
package ports

import (
	"context"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/library/domain"
)

// LibraryRepository stores readers' library items. Every method is scoped to
// the reader's own items.
type LibraryRepository interface {
	// Add puts the work in the reader's library, or returns the item if it
	// is already there.
	Add(ctx context.Context, userID, workID string) (domain.Item, error)
	Get(ctx context.Context, userID, id string) (domain.Item, error)
	List(ctx context.Context, userID string) ([]domain.Item, error)
	// FindByTitle matches a reader's item by case-insensitive work title.
	FindByTitle(ctx context.Context, userID, title string) (domain.Item, error)
	Update(ctx context.Context, userID, id string, p domain.Patch) (domain.Item, error)
}

// FileStorage holds readers' private copies of their books.
type FileStorage interface {
	// SignedURL returns a URL that downloads the file at path for ttl.
	SignedURL(ctx context.Context, path string, ttl time.Duration) (string, error)
}
