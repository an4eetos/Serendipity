// Package postgres implements the profile context's repositories on
// Postgres.
package postgres

import (
	"context"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

const profileColumns = `id, username, display_name, bio, is_public, terms_version, terms_accepted_at, created_at`

func scanProfile(row interface{ Scan(...any) error }) (domain.Profile, error) {
	var p domain.Profile
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Bio, &p.IsPublic, &p.TermsVersion, &p.TermsAcceptedAt,
		&p.CreatedAt)
	return p, err
}

// ProfileRepository implements ports.ProfileRepository.
type ProfileRepository struct {
	db *kernelpg.DB
}

func NewProfileRepository(db *kernelpg.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) Get(ctx context.Context, id string) (domain.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, `select `+profileColumns+` from profiles where id = $1`, id))
}

func (r *ProfileRepository) GetByUsername(ctx context.Context, username string) (domain.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, `select `+profileColumns+` from profiles where username = $1`, username))
}

func (r *ProfileRepository) Update(ctx context.Context, id string, p domain.Patch) (domain.Profile, error) {
	prof, err := scanProfile(r.db.QueryRow(ctx, `
		update profiles set
			username     = coalesce($2, username),
			display_name = coalesce($3, display_name),
			bio          = coalesce($4, bio),
			is_public    = coalesce($5, is_public)
		where id = $1
		returning `+profileColumns,
		id, p.Username, p.DisplayName, p.Bio, p.IsPublic))
	switch {
	case kernelpg.IsUniqueViolation(err):
		return domain.Profile{}, domain.ErrUsernameTaken
	case kernelpg.IsCheckViolation(err):
		return domain.Profile{}, domain.ErrInvalidField
	}
	return prof, err
}

func (r *ProfileRepository) AcceptTerms(ctx context.Context, id, version string) (domain.Profile, error) {
	return scanProfile(r.db.QueryRow(ctx, `
		update profiles set terms_version = $2, terms_accepted_at = now()
		where id = $1
		returning `+profileColumns,
		id, version))
}
