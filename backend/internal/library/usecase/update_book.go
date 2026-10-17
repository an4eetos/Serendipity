package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/ports"
)

// UpdateBook records reading progress, status or an uploaded file.
type UpdateBook struct {
	txm     kernelports.TxManager
	library ports.LibraryRepository
}

func NewUpdateBook(txm kernelports.TxManager, library ports.LibraryRepository) *UpdateBook {
	return &UpdateBook{txm: txm, library: library}
}

func (uc *UpdateBook) Execute(ctx context.Context, userID, id string, p domain.Patch) (domain.Item, error) {
	if err := p.Validate(userID); err != nil {
		return domain.Item{}, err
	}
	var item domain.Item
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		item, err = uc.library.Update(ctx, userID, id, p)
		return err
	})
	return item, err
}
