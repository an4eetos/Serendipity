package usecase

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
	"github.com/an4eetos/serendipity/backend/internal/moderation/ports"
)

// CheckAdmin lets only admins act as moderators.
type CheckAdmin struct {
	admins ports.AdminRepository
}

// NewCheckAdmin returns the CheckAdmin usecase.
func NewCheckAdmin(admins ports.AdminRepository) *CheckAdmin {
	return &CheckAdmin{admins: admins}
}

// Execute returns domain.ErrNotAdmin unless userID is an admin.
func (uc *CheckAdmin) Execute(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrNotAdmin
	}
	ok, err := uc.admins.IsAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotAdmin
	}
	return nil
}
