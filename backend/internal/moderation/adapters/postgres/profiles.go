package postgres

import (
	"context"
	"errors"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// ProfileVisibility implements ports.ProfileVisibility.
type ProfileVisibility struct {
	db *kernelpg.DB
}

// NewProfileVisibility returns a ProfileVisibility on db.
func NewProfileVisibility(db *kernelpg.DB) *ProfileVisibility {
	return &ProfileVisibility{db: db}
}

// IsPublic implements ports.ProfileVisibility.
func (r *ProfileVisibility) IsPublic(ctx context.Context, userID string) (bool, error) {
	var public bool
	err := r.db.QueryRow(ctx, `select is_public from profiles where id = $1`, userID).Scan(&public)
	if errors.Is(err, kernel.ErrNotFound) {
		return false, nil
	}
	return public, err
}
