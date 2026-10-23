package postgres

import (
	"context"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
)

// AdminRepository implements ports.AdminRepository.
type AdminRepository struct {
	db *kernelpg.DB
}

// NewAdminRepository returns an AdminRepository on db.
func NewAdminRepository(db *kernelpg.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// IsAdmin implements ports.AdminRepository.
func (r *AdminRepository) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `select exists (select 1 from admins where user_id = $1)`, userID).Scan(&ok)
	return ok, err
}
