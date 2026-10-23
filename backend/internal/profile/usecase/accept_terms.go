package usecase

import (
	"context"

	kernelports "github.com/an4eetos/serendipity/backend/internal/kernel/ports"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
)

// AcceptTerms records that the signed-in reader accepted the current terms of
// service.
type AcceptTerms struct {
	txm      kernelports.TxManager
	profiles ports.ProfileRepository
	terms    domain.Terms
}

func NewAcceptTerms(txm kernelports.TxManager, profiles ports.ProfileRepository, terms domain.Terms) *AcceptTerms {
	return &AcceptTerms{txm: txm, profiles: profiles, terms: terms}
}

// Execute accepts version, which must be the current one: the reader accepts
// the terms they were shown.
func (uc *AcceptTerms) Execute(ctx context.Context, userID, version string) (domain.Me, error) {
	if err := uc.terms.CheckAcceptable(version); err != nil {
		return domain.Me{}, err
	}
	var p domain.Profile
	err := uc.txm.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		p, err = uc.profiles.AcceptTerms(ctx, userID, version)
		return err
	})
	if err != nil {
		return domain.Me{}, err
	}
	return domain.NewMe(p, uc.terms), nil
}
