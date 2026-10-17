// Package postgres implements the library's repositories on Postgres.
package postgres

import (
	"context"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/library/domain"
)

const itemColumns = `li.id, ` + catalogpg.WorkColumns + `, li.file_path, li.file_type, li.progress, li.status,
	li.last_location, (select count(*) from quotes q where q.library_item_id = li.id), li.created_at, li.updated_at`

const itemFrom = ` from library_items li join works w on w.id = li.work_id `

func scanItem(row interface{ Scan(...any) error }) (domain.Item, error) {
	var li domain.Item
	dest := append([]any{&li.ID}, catalogpg.WorkDest(&li.Work)...)
	dest = append(dest, &li.FilePath, &li.FileType, &li.Progress, &li.Status,
		&li.LastLocation, &li.QuoteCount, &li.CreatedAt, &li.UpdatedAt)
	err := row.Scan(dest...)
	return li, err
}

// LibraryRepository implements ports.LibraryRepository.
type LibraryRepository struct {
	db *kernelpg.DB
}

func NewLibraryRepository(db *kernelpg.DB) *LibraryRepository {
	return &LibraryRepository{db: db}
}

func (r *LibraryRepository) Add(ctx context.Context, userID, workID string) (domain.Item, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		insert into library_items (user_id, work_id) values ($1, $2)
		on conflict (user_id, work_id) do update set updated_at = now()
		returning id`, userID, workID).Scan(&id)
	if err != nil {
		return domain.Item{}, err
	}
	return r.Get(ctx, userID, id)
}

func (r *LibraryRepository) Get(ctx context.Context, userID, id string) (domain.Item, error) {
	return scanItem(r.db.QueryRow(ctx,
		`select `+itemColumns+itemFrom+`where li.user_id = $1 and li.id = $2`, userID, id))
}

func (r *LibraryRepository) List(ctx context.Context, userID string) ([]domain.Item, error) {
	rows, err := r.db.Query(ctx,
		`select `+itemColumns+itemFrom+`where li.user_id = $1 order by li.updated_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Item{}
	for rows.Next() {
		li, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

func (r *LibraryRepository) FindByTitle(ctx context.Context, userID, title string) (domain.Item, error) {
	return scanItem(r.db.QueryRow(ctx,
		`select `+itemColumns+itemFrom+`where li.user_id = $1 and lower(w.title) = lower($2)
		order by li.created_at limit 1`, userID, title))
}

func (r *LibraryRepository) Update(ctx context.Context, userID, id string, p domain.Patch) (domain.Item, error) {
	tag, err := r.db.Exec(ctx, `
		update library_items set
			progress      = coalesce($3, progress),
			status        = coalesce($4::reading_status, status),
			last_location = coalesce($5, last_location),
			file_path     = coalesce($6, file_path),
			file_type     = coalesce($7::file_type, file_type),
			updated_at    = now()
		where user_id = $1 and id = $2`,
		userID, id, p.Progress, p.Status, p.LastLocation, p.FilePath, p.FileType)
	if err != nil {
		return domain.Item{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Item{}, kernel.ErrNotFound
	}
	return r.Get(ctx, userID, id)
}
