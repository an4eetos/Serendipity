// Package ports declares what the profile context needs from the outside.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

// ProfileRepository stores profiles. Every reader gets one on sign-up.
type ProfileRepository interface {
	Get(ctx context.Context, id string) (domain.Profile, error)
	GetByUsername(ctx context.Context, username string) (domain.Profile, error)
	// Update applies p. It returns domain.ErrUsernameTaken or
	// domain.ErrInvalidField when the profile would break a constraint.
	Update(ctx context.Context, id string, p domain.Patch) (domain.Profile, error)
	// AcceptTerms records that the reader accepted version now.
	AcceptTerms(ctx context.Context, id, version string) (domain.Profile, error)
}

// ShelfRepository reads what a profile shows of the reader's library.
type ShelfRepository interface {
	// Books lists the reader's books, most recently updated first, with the
	// count and reading trail of each book's listed quotes.
	Books(ctx context.Context, userID string) ([]domain.PublicBook, error)
	// TopCategories lists the subjects most of the reader's books share.
	TopCategories(ctx context.Context, userID string, limit int) ([]domain.Category, error)
}
