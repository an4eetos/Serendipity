package postgres

import (
	"context"

	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/moderation/domain"
)

const takedownColumns = `t.id, t.target_kind, t.target_id, t.target_owner_id, p.username, t.reason,
	t.hidden_by, t.restored_by, t.created_at, t.restored_at`

const takedownFrom = ` from takedowns t join profiles p on p.id = t.target_owner_id `

func scanTakedown(row interface{ Scan(...any) error }) (domain.Takedown, error) {
	var t domain.Takedown
	err := row.Scan(&t.ID, &t.Target.Kind, &t.Target.ID, &t.OwnerID, &t.OwnerUsername, &t.Reason,
		&t.HiddenBy, &t.RestoredBy, &t.CreatedAt, &t.RestoredAt)
	return t, err
}

// TakedownRepository implements ports.TakedownRepository.
type TakedownRepository struct {
	db *kernelpg.DB
}

// NewTakedownRepository returns a TakedownRepository on db.
func NewTakedownRepository(db *kernelpg.DB) *TakedownRepository {
	return &TakedownRepository{db: db}
}

// Create implements ports.TakedownRepository.
func (r *TakedownRepository) Create(ctx context.Context, t domain.Takedown) (domain.Takedown, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		insert into takedowns (target_kind, target_id, target_owner_id, reason, hidden_by)
		values ($1, $2, $3, $4, $5)
		returning id`,
		t.Target.Kind, t.Target.ID, t.OwnerID, t.Reason, t.HiddenBy,
	).Scan(&id)
	switch {
	case kernelpg.IsUniqueViolation(err):
		return domain.Takedown{}, domain.ErrAlreadyRemoved
	case err != nil:
		return domain.Takedown{}, err
	default:
		return r.Get(ctx, id)
	}
}

// Get implements ports.TakedownRepository.
func (r *TakedownRepository) Get(ctx context.Context, id string) (domain.Takedown, error) {
	return scanTakedown(r.db.QueryRow(ctx, `select `+takedownColumns+takedownFrom+`where t.id = $1`, id))
}

// ListActive implements ports.TakedownRepository.
func (r *TakedownRepository) ListActive(ctx context.Context, limit int) ([]domain.Takedown, error) {
	rows, err := r.db.Query(ctx, `select `+takedownColumns+takedownFrom+`
		where t.restored_at is null
		order by t.created_at desc
		limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Takedown{}
	for rows.Next() {
		t, err := scanTakedown(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MarkRestored implements ports.TakedownRepository.
func (r *TakedownRepository) MarkRestored(ctx context.Context, id, adminID string) (domain.Takedown, error) {
	tag, err := r.db.Exec(ctx, `
		update takedowns set
			restored_at = now(),
			restored_by = $2,
			updated_at  = now()
		where id = $1 and restored_at is null`, id, adminID)
	if err != nil {
		return domain.Takedown{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Takedown{}, domain.ErrAlreadyRestored
	}
	return r.Get(ctx, id)
}
