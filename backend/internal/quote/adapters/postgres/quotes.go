// Package postgres implements the quote context's repositories on Postgres.
package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/domain"
	"github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

const quoteColumns = `q.id, q.user_id, q.work_id, q.library_item_id, q.text, q.location, q.anchor, q.position,
	q.reaction, q.source, q.visibility, q.passage_hidden, q.passage_words, q.removed_at, q.removed_reason,
	q.created_at, q.updated_at, ` + catalogpg.WorkColumns

const quoteFrom = ` from quotes q join works w on w.id = q.work_id `

func scanQuote(row interface{ Scan(...any) error }) (domain.Quote, error) {
	var q domain.Quote
	dest := []any{&q.ID, &q.UserID, &q.WorkID, &q.LibraryItemID, &q.Text, &q.Location, &q.Anchor, &q.Position,
		&q.Reaction, &q.Source, &q.Visibility, &q.PassageHidden, &q.PassageWords, &q.RemovedAt, &q.RemovedReason,
		&q.CreatedAt, &q.UpdatedAt}
	dest = append(dest, catalogpg.WorkDest(&q.Work)...)
	err := row.Scan(dest...)
	return q, err
}

// QuoteRepository implements ports.QuoteRepository and, for moderators,
// ports.QuoteModeration.
type QuoteRepository struct {
	db *kernelpg.DB
}

var (
	_ ports.QuoteRepository = (*QuoteRepository)(nil)
	_ ports.QuoteModeration = (*QuoteRepository)(nil)
)

func NewQuoteRepository(db *kernelpg.DB) *QuoteRepository {
	return &QuoteRepository{db: db}
}

// coverageTrigger is the function behind trigger quotes_enforce_public_coverage.
// Its only error is the check violation it raises when a removed quote is made
// public (migration 20261024000000_phase1_open_to_strangers.sql).
const coverageTrigger = "public.enforce_public_coverage"

// isRemovedQuoteGuard reports whether err is that check violation, and not a
// violation of one of the table's check constraints, which carry a name.
func isRemovedQuoteGuard(err error) bool {
	var pgErr *pgconn.PgError
	return kernelpg.IsCheckViolation(err) && errors.As(err, &pgErr) && pgErr.ConstraintName == "" &&
		strings.Contains(pgErr.Where, "PL/pgSQL function "+coverageTrigger+"(")
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
		where q.user_id = $1 and q.visibility = 'public' and q.removed_at is null
		order by q.created_at desc limit $2`, userID, limit)
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
			reaction       = coalesce($3, reaction),
			visibility     = coalesce($4::visibility, visibility),
			passage_hidden = coalesce($5, passage_hidden),
			updated_at     = now()
		where user_id = $1 and id = $2`, userID, id, p.Reaction, p.Visibility, p.PassageHidden)
	if isRemovedQuoteGuard(err) {
		// A takedown committed after the caller read the quote.
		return domain.Quote{}, domain.ErrRemoved
	}
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

// ListAny returns the quotes with these ids, whoever owns them.
func (r *QuoteRepository) ListAny(ctx context.Context, ids []string) ([]domain.Quote, error) {
	if len(ids) == 0 {
		return []domain.Quote{}, nil
	}
	return r.list(ctx, `select `+quoteColumns+quoteFrom+`where q.id = any($1::uuid[])`, ids)
}

func (r *QuoteRepository) Remove(ctx context.Context, id, reason string) error {
	tag, err := r.db.Exec(ctx, `
		update quotes set removed_at = now(), removed_reason = $2, updated_at = now()
		where id = $1 and removed_at is null`, id, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrNotFound
	}
	return nil
}

// Restore runs in the caller's transaction, so the cap it sets is the one the
// coverage trigger applies to its update.
func (r *QuoteRepository) Restore(ctx context.Context, id string, wordCap domain.CoverageCap) error {
	if err := setCoverageCap(ctx, r.db, wordCap); err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `
		update quotes set removed_at = null, removed_reason = '', updated_at = now()
		where id = $1 and removed_at is not null`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return kernel.ErrNotFound
	}
	return nil
}
