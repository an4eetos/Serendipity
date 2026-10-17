package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// DeleteQuote removes one of the reader's quotes with its discussions.
type DeleteQuote struct {
	txm    kernelports.TxManager
	quotes ports.QuoteRepository
}

func NewDeleteQuote(txm kernelports.TxManager, quotes ports.QuoteRepository) *DeleteQuote {
	return &DeleteQuote{txm: txm, quotes: quotes}
}

func (uc *DeleteQuote) Execute(ctx context.Context, userID, id string) error {
	return uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		return uc.quotes.Delete(ctx, userID, id)
	})
}
