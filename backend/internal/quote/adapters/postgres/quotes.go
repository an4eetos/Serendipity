// Package postgres implements the quote context's repositories on Postgres.
package postgres

import (
	"context"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

const quoteColumns = `q.id, q.work_id, q.library_item_id, q.text, q.location, q.anchor, q.position,
	q.reaction, q.source, q.visibility, q.created_at, q.updated_at, ` + catalogpg.WorkColumns

const quoteFrom = ` from quotes q join works w on w.id = q.work_id `

func scanQuote(row interface{ Scan(...any) error }) (domain.Quote, error) {
	var q domain.Quote
	dest := []any{&q.ID, &q.WorkID, &q.LibraryItemID, &q.Text, &q.Location, &q.Anchor, &q.Position,
		&q.Reaction, &q.Source, &q.Visibility, &q.CreatedAt, &q.UpdatedAt}
	dest = append(dest, catalogpg.WorkDest(&q.Work)...)
	err := row.Scan(dest...)
	return q, err
}

// QuoteRepository implements ports.QuoteRepository.
type QuoteRepository struct {
	db *kernelpg.DB
}

func NewQuoteRepository(db *kernelpg.DB) *QuoteRepository {
	return &QuoteRepository{db: db}
}

func (r *QuoteRepository) Create(ctx context.Context, userID string, d domain.Draft) (domain.Quote, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		insert into quotes (user_id, work_id, library_item_id, text, location, anchor, position, reaction, source)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::quote_source)
		on conflict (user_id, work_id, md5(text)) do update set updated_at = quotes.updated_at
		returning id`,
		userID, d.WorkID, d.LibraryItemID, d.Text, d.Location, d.Anchor, d.Position, d.Reaction, d.Source).Scan(&id)
	if err != nil {
		return domain.Quote{}, err
	}
	return r.Get(ctx, userID, id)
}

func (r *QuoteRepository) Import(ctx context.Context, userID string, d domain.Draft) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		insert into quotes (user_id, work_id, library_item_id, text, location, reaction, source)
		values ($1, $2, $3, $4, $5, $6, $7::quote_source)
		on conflict (user_id, work_id, md5(text)) do nothing`,
		userID, d.WorkID, d.LibraryItemID, d.Text, d.Location, d.Reaction, d.Source)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *QuoteRepository) Get(ctx context.Context, userID, id string) (domain.Quote, error) {
	return scanQuote(r.db.QueryRow(ctx, `select `+quoteColumns+quoteFrom+`where q.user_id = $1 and q.id = $2`, userID, id))
}

func (r *QuoteRepository) List(ctx context.Context, userID, workID string) ([]domain.Quote, error) {
	return r.list(ctx, `select `+quoteColumns+quoteFrom+`
		where q.user_id = $1 and ($2 = '' or q.work_id::text = $2)
		order by q.position nulls last, q.created_at`, userID, workID)
}

func (r *QuoteRepository) ListPublic(ctx context.Context, userID string, limit int) ([]domain.Quote, error) {
	return r.list(ctx, `select `+quoteColumns+quoteFrom+`
		where q.user_id = $1 and q.visibility = 'public' order by q.created_at desc limit $2`, userID, limit)
}

func (r *QuoteRepository) list(ctx context.Context, sql string, args ...any) ([]domain.Quote, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Quote{}
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *QuoteRepository) Update(ctx context.Context, userID, id string, p domain.Patch) (domain.Quote, error) {
	tag, err := r.db.Exec(ctx, `
		update quotes set
			reaction   = coalesce($3, reaction),
			visibility = coalesce($4::visibility, visibility),
			updated_at = now()
		where user_id = $1 and id = $2`, userID, id, p.Reaction, p.Visibility)
	if err != nil {
		return domain.Quote{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Quote{}, kernel.ErrNotFound
	}
	return r.Get(ctx, userID, id)
}

func (r *QuoteRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `delete from quotes where user_id = $1 and id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrNotFound
	}
	return nil
}
