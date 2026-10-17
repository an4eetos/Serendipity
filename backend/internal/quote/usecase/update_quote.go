package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// UpdateQuote edits the reader's reaction or publishes the quote.
type UpdateQuote struct {
	txm    kernelports.TxManager
	quotes ports.QuoteRepository
}

func NewUpdateQuote(txm kernelports.TxManager, quotes ports.QuoteRepository) *UpdateQuote {
	return &UpdateQuote{txm: txm, quotes: quotes}
}

func (uc *UpdateQuote) Execute(ctx context.Context, userID, id string, p domain.Patch) (domain.Quote, error) {
	current, err := uc.quotes.Get(ctx, userID, id)
	if err != nil {
		return domain.Quote{}, err
	}
	if err := current.CheckPatch(p); err != nil {
		return domain.Quote{}, err
	}
	var q domain.Quote
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		q, err = uc.quotes.Update(ctx, userID, id, p)
		return err
	})
	return q, err
}
