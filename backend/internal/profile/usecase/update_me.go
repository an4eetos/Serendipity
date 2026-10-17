package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
)

// UpdateMe edits the signed-in reader's own profile.
type UpdateMe struct {
	txm      kernelports.TxManager
	profiles ports.ProfileRepository
}

func NewUpdateMe(txm kernelports.TxManager, profiles ports.ProfileRepository) *UpdateMe {
	return &UpdateMe{txm: txm, profiles: profiles}
}

func (uc *UpdateMe) Execute(ctx context.Context, userID string, p domain.Patch) (domain.Profile, error) {
	p, err := p.Normalize()
	if err != nil {
		return domain.Profile{}, err
	}
	var prof domain.Profile
	err = uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		prof, err = uc.profiles.Update(ctx, userID, p)
		return err
	})
	return prof, err
}
