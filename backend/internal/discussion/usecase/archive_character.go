package usecase

import (
	"context"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// ArchiveCharacter archives a reader's own character: it leaves the picker
// and its discussions become read-only.
type ArchiveCharacter struct {
	txm   kernelports.TxManager
	chars ports.CharacterRepository
}

// NewArchiveCharacter returns the ArchiveCharacter usecase.
func NewArchiveCharacter(txm kernelports.TxManager, chars ports.CharacterRepository) *ArchiveCharacter {
	return &ArchiveCharacter{txm: txm, chars: chars}
}

// Execute archives ownerID's character id. Archiving an archived character
// changes nothing.
func (uc *ArchiveCharacter) Execute(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	// The id comes from the reader: check it before taking a row lock.
	if _, err := uc.chars.GetOwn(ctx, ownerID, id); err != nil {
		return domain.Persona{}, err
	}
	var archived domain.Persona
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		p, err := uc.chars.GetOwnForUpdate(ctx, ownerID, id)
		if err != nil {
			return err
		}
		wasArchived := p.Archived()
		next, err := p.Archive(time.Now())
		if err != nil {
			return err
		}
		if wasArchived {
			archived = p
			return nil
		}
		archived, err = uc.chars.Update(ctx, next)
		return err
	})
	if err != nil {
		return domain.Persona{}, err
	}
	return archived, nil
}
