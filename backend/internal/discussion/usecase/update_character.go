package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// UpdateCharacter edits a reader's own character.
type UpdateCharacter struct {
	txm   kernelports.TxManager
	chars ports.CharacterRepository
}

// NewUpdateCharacter returns the UpdateCharacter usecase.
func NewUpdateCharacter(txm kernelports.TxManager, chars ports.CharacterRepository) *UpdateCharacter {
	return &UpdateCharacter{txm: txm, chars: chars}
}

// Execute applies patch to ownerID's character id.
func (uc *UpdateCharacter) Execute(ctx context.Context, ownerID, id string, patch domain.CharacterPatch) (domain.Persona, error) {
	// The id comes from the reader: check it before taking a row lock.
	if _, err := uc.chars.GetOwn(ctx, ownerID, id); err != nil {
		return domain.Persona{}, err
	}
	var updated domain.Persona
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		p, err := uc.chars.GetOwnForUpdate(ctx, ownerID, id)
		if err != nil {
			return err
		}
		edited, err := p.Edit(patch)
		if err != nil {
			return err
		}
		updated, err = uc.chars.Update(ctx, edited)
		return err
	})
	if err != nil {
		return domain.Persona{}, err
	}
	return updated, nil
}
