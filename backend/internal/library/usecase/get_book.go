package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

// GetBook returns one item of the reader's library.
type GetBook struct {
	library ports.LibraryRepository
}

func NewGetBook(library ports.LibraryRepository) *GetBook {
	return &GetBook{library: library}
}

func (uc *GetBook) Execute(ctx context.Context, userID, id string) (domain.Item, error) {
	return uc.library.Get(ctx, userID, id)
}
