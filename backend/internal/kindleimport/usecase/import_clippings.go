// Package usecase holds the Kindle import's business actions.
package usecase

import (
	"context"
	"errors"
	"io"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	catalogports "github.com/an4eetos/serendipity/backend/internal/catalog/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/kindleimport/domain"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	libraryports "github.com/an4eetos/serendipity/backend/internal/library/ports"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

// ImportClippings brings a Kindle's highlights into the reader's library,
// all or nothing: each book becomes a library item, catalogued if it is new,
// and each highlight a quote. Re-importing the same file adds nothing. The
// reader must confirm they own the copies, and each book records it.
type ImportClippings struct {
	txm     kernelports.TxManager
	works   catalogports.WorkRepository
	library libraryports.LibraryRepository
	quotes  quoteports.QuoteRepository
}

func NewImportClippings(
	txm kernelports.TxManager,
	works catalogports.WorkRepository,
	library libraryports.LibraryRepository,
	quotes quoteports.QuoteRepository,
) *ImportClippings {
	return &ImportClippings{txm: txm, works: works, library: library, quotes: quotes}
}

func (uc *ImportClippings) Execute(ctx context.Context, userID string, clippings io.Reader,
	ownershipConfirmed bool) (domain.Report, error) {
	if err := library.CheckOwnershipConfirmed(ownershipConfirmed); err != nil {
		return domain.Report{}, err
	}
	books, err := domain.Parse(clippings)
	if err != nil {
		return domain.Report{}, err
	}
	var report domain.Report
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		report = domain.Report{}
		for _, b := range books {
			item, err := uc.libraryItem(ctx, userID, b)
			if err != nil {
				return err
			}
			if _, err := uc.library.Update(ctx, userID, item.ID,
				library.Patch{OwnershipConfirmed: true}); err != nil {
				return err
			}
			report.Books++
			report.OrphanNotes += b.OrphanNotes
			for _, h := range b.Highlights {
				if quote.ValidateText(h.Text) != nil {
					report.SkippedTooLong++
					continue
				}
				inserted, err := uc.quotes.Import(ctx, userID, h.Draft(item.Work.ID, item.ID))
				if err != nil {
					return err
				}
				if inserted {
					report.Imported++
				} else {
					report.Duplicates++
				}
			}
		}
		return nil
	})
	return report, err
}

// libraryItem finds the book in the reader's library by title, adding it if
// it is not there yet.
func (uc *ImportClippings) libraryItem(ctx context.Context, userID string, b domain.Book) (library.Item, error) {
	item, err := uc.library.FindByTitle(ctx, userID, b.Title)
	if !errors.Is(err, kernel.ErrNotFound) {
		return item, err
	}
	work, err := catalog.NewWork(catalog.Work{Title: b.Title, Authors: b.Authors()})
	if err != nil {
		return library.Item{}, err
	}
	stored, err := uc.works.Ensure(ctx, work, userID)
	if err != nil {
		return library.Item{}, err
	}
	return uc.library.Add(ctx, userID, stored.ID)
}
