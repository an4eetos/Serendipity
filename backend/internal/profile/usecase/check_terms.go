package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
)

// CheckTerms checks that the signed-in reader accepted the current terms of
// service, which they must before they change anything.
type CheckTerms struct {
	profiles ports.ProfileRepository
	terms    domain.Terms
}

func NewCheckTerms(profiles ports.ProfileRepository, terms domain.Terms) *CheckTerms {
	return &CheckTerms{profiles: profiles, terms: terms}
}

// Execute returns domain.ErrTermsNotAccepted unless the reader accepted the
// current version.
func (uc *CheckTerms) Execute(ctx context.Context, userID string) error {
	p, err := uc.profiles.Get(ctx, userID)
	if err != nil {
		return err
	}
	return uc.terms.CheckAccepted(p)
}
