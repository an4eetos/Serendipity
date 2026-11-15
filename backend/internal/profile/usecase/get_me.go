// Package usecase holds the profile context's business actions.
package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
)

// GetMe returns the signed-in reader's own profile and whether they accepted
// the current terms of service.
type GetMe struct {
	profiles ports.ProfileRepository
	terms    domain.Terms
}

// NewGetMe returns the GetMe usecase for the current terms.
func NewGetMe(profiles ports.ProfileRepository, terms domain.Terms) *GetMe {
	return &GetMe{profiles: profiles, terms: terms}
}

// Execute returns the reader's own profile and whether they accepted the
// current terms.
func (uc *GetMe) Execute(ctx context.Context, userID string) (domain.Me, error) {
	p, err := uc.profiles.Get(ctx, userID)
	if err != nil {
		return domain.Me{}, err
	}
	return domain.NewMe(p, uc.terms), nil
}
