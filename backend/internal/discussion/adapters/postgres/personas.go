// Package postgres implements the discussion context's repositories on
// Postgres.
package postgres

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

const personaColumns = `p.id, p.slug, p.name, p.era, p.short_bio, p.system_prompt, p.is_preset,
	coalesce(p.owner_id::text, ''), p.archived_at, p.removed_at, p.removed_reason, p.created_at, p.updated_at`

// personaDest scans the same 13 columns as personaColumns into p.
func personaDest(p *domain.Persona) []any {
	return []any{&p.ID, &p.Slug, &p.Name, &p.Era, &p.ShortBio, &p.SystemPrompt, &p.IsPreset,
		&p.OwnerID, &p.ArchivedAt, &p.RemovedAt, &p.RemovedReason, &p.CreatedAt, &p.UpdatedAt}
}

// usedColumn is whether any conversation uses the persona: its name is then
// locked.
const usedColumn = `exists (select 1 from conversations c where c.persona_id = p.id)`

// inPublicDiscussionColumn is whether a discussion public pages would show
// uses the persona: the quote is public and not removed, the conversation
// is not removed and has an answer.
const inPublicDiscussionColumn = `exists (select 1 from conversations c join quotes q on q.id = c.quote_id
	where c.persona_id = p.id and c.removed_at is null and q.visibility = 'public' and q.removed_at is null
	and exists (select 1 from messages m where m.conversation_id = c.id and m.role = 'assistant'))`

// PersonaRepository implements ports.PersonaRepository, and
// ports.CharacterRepository for readers' own characters and
// ports.PersonaModeration for moderators.
type PersonaRepository struct {
	db *kernelpg.DB
}

var (
	_ ports.PersonaRepository   = (*PersonaRepository)(nil)
	_ ports.CharacterRepository = (*PersonaRepository)(nil)
	_ ports.PersonaModeration   = (*PersonaRepository)(nil)
)

func NewPersonaRepository(db *kernelpg.DB) *PersonaRepository {
	return &PersonaRepository{db: db}
}

func (r *PersonaRepository) List(ctx context.Context, userID string) ([]domain.Persona, error) {
	return r.query(ctx, false, `select `+personaColumns+` from personas p
		where p.is_preset or (p.owner_id = $1 and p.archived_at is null and p.removed_at is null)
		order by p.is_preset desc, p.sort_order, p.name`, userID)
}

func (r *PersonaRepository) Get(ctx context.Context, userID, id string) (domain.Persona, error) {
	var p domain.Persona
	err := r.db.QueryRow(ctx, `select `+personaColumns+` from personas p
		where p.id = $2 and (p.is_preset or p.owner_id = $1)`, userID, id).Scan(personaDest(&p)...)
	return p, err
}

// ListOwn implements ports.CharacterRepository.
func (r *PersonaRepository) ListOwn(ctx context.Context, ownerID string) ([]domain.Persona, error) {
	return r.query(ctx, true, `select `+personaColumns+`, `+usedColumn+` from personas p
		where p.owner_id = $1 order by p.created_at desc`, ownerID)
}

// GetOwn implements ports.CharacterRepository.
func (r *PersonaRepository) GetOwn(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	var p domain.Persona
	err := r.db.QueryRow(ctx, `select `+personaColumns+`, `+usedColumn+` from personas p
		where p.owner_id = $1 and p.id = $2`, ownerID, id).Scan(append(personaDest(&p), &p.HasConversations)...)
	return p, err
}

// GetOwnForUpdate implements ports.CharacterRepository. The usage check runs
// after the row lock, so it sees conversations committed while it waited.
func (r *PersonaRepository) GetOwnForUpdate(ctx context.Context, ownerID, id string) (domain.Persona, error) {
	var p domain.Persona
	err := r.db.QueryRow(ctx, `select `+personaColumns+` from personas p
		where p.owner_id = $1 and p.id = $2 for no key update`, ownerID, id).Scan(personaDest(&p)...)
	if err != nil {
		return domain.Persona{}, err
	}
	err = r.db.QueryRow(ctx, `select exists (select 1 from conversations where persona_id = $1)`, p.ID).
		Scan(&p.HasConversations)
	if err != nil {
		return domain.Persona{}, err
	}
	return p, nil
}

// LockOwner implements ports.CharacterRepository by locking the owner's
// profile row.
func (r *PersonaRepository) LockOwner(ctx context.Context, ownerID string) error {
	var one int
	return r.db.QueryRow(ctx, `select 1 from profiles where id = $1 for no key update`, ownerID).Scan(&one)
}

// CountActive implements ports.CharacterRepository.
func (r *PersonaRepository) CountActive(ctx context.Context, ownerID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `select count(*) from personas
		where owner_id = $1 and archived_at is null and removed_at is null`, ownerID).Scan(&n)
	return n, err
}

// Create implements ports.CharacterRepository. The slug is slugBase plus 10
// random hex digits.
func (r *PersonaRepository) Create(ctx context.Context, ownerID string, d domain.CharacterDraft, slugBase string) (domain.Persona, error) {
	var p domain.Persona
	err := r.db.QueryRow(ctx, `
		insert into personas as p (slug, name, era, short_bio, system_prompt, is_preset, owner_id)
		values ($1 || '-' || substr(md5(gen_random_uuid()::text), 1, 10), $2, $3, $4, $5, false, $6)
		returning `+personaColumns, slugBase, d.Name, d.Era, d.ShortBio, d.Prompt, ownerID).Scan(personaDest(&p)...)
	return p, err
}

// Update implements ports.CharacterRepository.
func (r *PersonaRepository) Update(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	var out domain.Persona
	err := r.db.QueryRow(ctx, `
		update personas as p set name = $3, era = $4, short_bio = $5, system_prompt = $6, archived_at = $7,
			updated_at = now()
		where p.id = $1 and p.owner_id = $2 and p.removed_at is null
		returning `+personaColumns+`, `+usedColumn,
		p.ID, p.OwnerID, p.Name, p.Era, p.ShortBio, p.SystemPrompt, p.ArchivedAt).
		Scan(append(personaDest(&out), &out.HasConversations)...)
	if kernelpg.IsCheckViolationOf(err, "personas_name_locked") {
		return domain.Persona{}, domain.ErrNameLocked
	}
	if err != nil {
		return domain.Persona{}, err
	}
	return out, nil
}

// ListAny implements ports.PersonaModeration.
func (r *PersonaRepository) ListAny(ctx context.Context, ids []string) ([]domain.ModeratedPersona, error) {
	if len(ids) == 0 {
		return []domain.ModeratedPersona{}, nil
	}
	rows, err := r.db.Query(ctx, `select `+personaColumns+`, `+inPublicDiscussionColumn+` from personas p
		where p.id = any($1::uuid[]) and not p.is_preset`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ModeratedPersona{}
	for rows.Next() {
		var m domain.ModeratedPersona
		if err := rows.Scan(append(personaDest(&m.Persona), &m.InPublicDiscussion)...); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Remove implements ports.PersonaModeration.
func (r *PersonaRepository) Remove(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx, `
		update personas set removed_at = now(), removed_reason = $2, updated_at = now()
		where id = $1 and not is_preset and removed_at is null`, id, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrNotFound
	}
	return nil
}

// Restore implements ports.PersonaModeration.
func (r *PersonaRepository) Restore(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `
		update personas set removed_at = null, removed_reason = '', updated_at = now()
		where id = $1 and removed_at is not null`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrNotFound
	}
	return nil
}

// query returns the personas sql selects, with HasConversations as an extra
// last column when used is set.
func (r *PersonaRepository) query(ctx context.Context, used bool, sql string, args ...any) ([]domain.Persona, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Persona{}
	for rows.Next() {
		var p domain.Persona
		dest := personaDest(&p)
		if used {
			dest = append(dest, &p.HasConversations)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
