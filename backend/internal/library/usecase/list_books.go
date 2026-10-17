package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

// ListBooks lists the reader's library, most recently updated first.
type ListBooks struct {
	library ports.LibraryRepository
}

func NewListBooks(library ports.LibraryRepository) *ListBooks {
	return &ListBooks{library: library}
}

func (uc *ListBooks) Execute(ctx context.Context, userID string) ([]domain.Item, error) {
	return uc.library.List(ctx, userID)
}
