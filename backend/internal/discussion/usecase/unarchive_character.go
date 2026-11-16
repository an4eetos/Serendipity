package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// UnarchiveCharacter makes a reader's archived character usable again,
// within their cap of active characters.
type UnarchiveCharacter struct {
	txm   kernelports.TxManager
	chars ports.CharacterRepository
	limit domain.CharacterLimit
}

// NewUnarchiveCharacter returns the UnarchiveCharacter usecase.
func NewUnarchiveCharacter(txm kernelports.TxManager, chars ports.CharacterRepository, limit domain.CharacterLimit) *UnarchiveCharacter {
	return &UnarchiveCharacter{txm: txm, chars: chars, limit: limit}
}

// Execute unarchives ownerID's character id. Unarchiving an active character
// changes nothing.
func (uc *UnarchiveCharacter) Execute(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	// The id comes from the reader: check it before taking any lock.
	if _, err := uc.chars.GetOwn(ctx, ownerID, id); err != nil {
		return domain.Persona{}, err
	}
	var unarchived domain.Persona
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		// Lock order: the owner before the character row, as in CreateCharacter.
		if err := uc.chars.LockOwner(ctx, ownerID); err != nil {
			return err
		}
		p, err := uc.chars.GetOwnForUpdate(ctx, ownerID, id)
		if err != nil {
			return err
		}
		if p.Removed() {
			return domain.ErrCharacterRemoved
		}
		if !p.Archived() {
			unarchived = p
			return nil
		}
		active, err := uc.chars.CountActive(ctx, ownerID)
		if err != nil {
			return err
		}
		if err := uc.limit.CheckRoom(active); err != nil {
			return err
		}
		next, err := p.Unarchive()
		if err != nil {
			return err
		}
		unarchived, err = uc.chars.Update(ctx, next)
		return err
	})
	if err != nil {
		return domain.Persona{}, err
	}
	return unarchived, nil
}
