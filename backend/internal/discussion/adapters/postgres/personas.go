// Package postgres implements the discussion context's repositories on
// Postgres.
package postgres

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
)

const personaColumns = `p.id, p.slug, p.name, p.era, p.short_bio, p.system_prompt, p.is_preset`

func personaDest(p *domain.Persona) []any {
	return []any{&p.ID, &p.Slug, &p.Name, &p.Era, &p.ShortBio, &p.SystemPrompt, &p.IsPreset}
}

// PersonaRepository implements ports.PersonaRepository.
type PersonaRepository struct {
	db *kernelpg.DB
}

func NewPersonaRepository(db *kernelpg.DB) *PersonaRepository {
	return &PersonaRepository{db: db}
}

func (r *PersonaRepository) List(ctx context.Context, userID string) ([]domain.Persona, error) {
	rows, err := r.db.Query(ctx, `select `+personaColumns+` from personas p
		where p.is_preset or p.owner_id = $1 order by p.is_preset desc, p.sort_order, p.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Persona{}
	for rows.Next() {
		var p domain.Persona
		if err := rows.Scan(personaDest(&p)...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PersonaRepository) Get(ctx context.Context, userID, id string) (domain.Persona, error) {
	var p domain.Persona
	err := r.db.QueryRow(ctx, `select `+personaColumns+` from personas p
		where p.id = $2 and (p.is_preset or p.owner_id = $1)`, userID, id).Scan(personaDest(&p)...)
	return p, err
}
