// Package usecase holds the profile context's business actions.
package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
	"github.com/an4eetos/serendipity/backend/internal/profile/ports"
)

// GetMe returns the signed-in reader's own profile.
type GetMe struct {
	profiles ports.ProfileRepository
}

func NewGetMe(profiles ports.ProfileRepository) *GetMe {
	return &GetMe{profiles: profiles}
}

func (uc *GetMe) Execute(ctx context.Context, userID string) (domain.Profile, error) {
	return uc.profiles.Get(ctx, userID)
}
