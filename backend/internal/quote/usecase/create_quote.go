// Package usecase holds the quote context's business actions.
package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	libraryports "github.com/an4eetos/serendipity/backend/internal/library/ports"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// CreateQuote saves a passage the reader took from a book in their library.
type CreateQuote struct {
	txm     kernelports.TxManager
	library libraryports.LibraryRepository
	quotes  ports.QuoteRepository
}

func NewCreateQuote(txm kernelports.TxManager, library libraryports.LibraryRepository, quotes ports.QuoteRepository) *CreateQuote {
	return &CreateQuote{txm: txm, library: library, quotes: quotes}
}

func (uc *CreateQuote) Execute(ctx context.Context, userID, libraryItemID string, d domain.Draft) (domain.Quote, error) {
	draft, err := domain.NewUpload(d)
	if err != nil {
		return domain.Quote{}, err
	}
	item, err := uc.library.Get(ctx, userID, libraryItemID)
	if err != nil {
		return domain.Quote{}, err
	}
	draft.WorkID, draft.LibraryItemID = item.Work.ID, &item.ID

	var q domain.Quote
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		q, err = uc.quotes.Create(ctx, userID, draft)
		return err
	})
	return q, err
}
