package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// ListQuotes lists the reader's quotes, of one work if workID is not empty.
type ListQuotes struct {
	quotes ports.QuoteRepository
}

func NewListQuotes(quotes ports.QuoteRepository) *ListQuotes {
	return &ListQuotes{quotes: quotes}
}

func (uc *ListQuotes) Execute(ctx context.Context, userID, workID string) ([]domain.Quote, error) {
	return uc.quotes.List(ctx, userID, workID)
}
