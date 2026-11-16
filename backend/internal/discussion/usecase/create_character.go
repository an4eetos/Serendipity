package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
)

// CreateCharacter creates a reader's own character, within their cap of
// active characters.
type CreateCharacter struct {
	txm   kernelports.TxManager
	chars ports.CharacterRepository
	limit domain.CharacterLimit
}

// NewCreateCharacter returns the CreateCharacter usecase.
func NewCreateCharacter(txm kernelports.TxManager, chars ports.CharacterRepository, limit domain.CharacterLimit) *CreateCharacter {
	return &CreateCharacter{txm: txm, chars: chars, limit: limit}
}

// Execute validates d and stores it as ownerID's new character.
func (uc *CreateCharacter) Execute(ctx context.Context, ownerID string, d domain.CharacterDraft) (domain.Persona, error) {
	d, err := d.Validate()
	if err != nil {
		return domain.Persona{}, err
	}
	var p domain.Persona
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		// The owner lock keeps concurrent creates from both passing the cap.
		if err := uc.chars.LockOwner(ctx, ownerID); err != nil {
			return err
		}
		active, err := uc.chars.CountActive(ctx, ownerID)
		if err != nil {
			return err
		}
		if err := uc.limit.CheckRoom(active); err != nil {
			return err
		}
		p, err = uc.chars.Create(ctx, ownerID, d, domain.SlugBase(d.Name))
		return err
	})
	if err != nil {
		return domain.Persona{}, err
	}
	return p, nil
}
