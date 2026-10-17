// Package usecase holds the library's business actions.
package usecase

import (
	"context"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	catalogports "github.com/an4eetos/serendipity/backend/internal/catalog/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

// AddBook puts a work in the reader's library, cataloguing it first if it is
// new to Serendipity.
type AddBook struct {
	txm     kernelports.TxManager
	works   catalogports.WorkRepository
	library ports.LibraryRepository
}

func NewAddBook(txm kernelports.TxManager, works catalogports.WorkRepository, library ports.LibraryRepository) *AddBook {
	return &AddBook{txm: txm, works: works, library: library}
}

func (uc *AddBook) Execute(ctx context.Context, userID string, draft catalog.Work) (domain.Item, error) {
	work, err := catalog.NewWork(draft)
	if err != nil {
		return domain.Item{}, err
	}
	var item domain.Item
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		stored, err := uc.works.Ensure(ctx, work, userID)
		if err != nil {
			return err
		}
		item, err = uc.library.Add(ctx, userID, stored.ID)
		return err
	})
	return item, err
}
