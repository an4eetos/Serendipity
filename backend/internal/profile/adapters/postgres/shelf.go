package postgres

import (
	"context"

	catalogpg "github.com/an4eetos/serendipity/backend/internal/catalog/adapters/postgres"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	"github.com/an4eetos/serendipity/backend/internal/profile/domain"
)

// ShelfRepository implements ports.ShelfRepository.
type ShelfRepository struct {
	db *kernelpg.DB
}

func NewShelfRepository(db *kernelpg.DB) *ShelfRepository {
	return &ShelfRepository{db: db}
}

// Books counts and trails only listed quotes (public, not removed). The
// trail is aggregated per book in the same query, so it costs no extra
// round trip per book.
func (r *ShelfRepository) Books(ctx context.Context, userID string) ([]domain.PublicBook, error) {
	rows, err := r.db.Query(ctx, `
		select `+catalogpg.WorkColumns+`, li.status, li.progress,
			coalesce(t.n, 0), coalesce(t.ids, '{}'::text[]), coalesce(t.positions, '{}'::float8[])
		from library_items li join works w on w.id = li.work_id
		left join lateral (
			select count(*) as n,
				array_agg(q.id::text order by q.position, q.created_at) filter (where q.position is not null) as ids,
				array_agg(q.position order by q.position, q.created_at) filter (where q.position is not null) as positions
			from quotes q
			where q.user_id = li.user_id and q.work_id = li.work_id
				and q.visibility = 'public' and q.removed_at is null
		) t on true
		where li.user_id = $1
		order by li.updated_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	books := []domain.PublicBook{}
	for rows.Next() {
		var b domain.PublicBook
		var ids []string
		var positions []float64
		dest := append(catalogpg.WorkDest(&b.Work), &b.Status, &b.Progress, &b.PublicQuotes, &ids, &positions)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		b.Trail = trail(ids, positions)
		books = append(books, b)
	}
	return books, rows.Err()
}

// trail pairs the aggregated quote ids with their positions, which the query
// orders the same way.
func trail(ids []string, positions []float64) []domain.TrailPoint {
	points := make([]domain.TrailPoint, min(len(ids), len(positions)))
	for i := range points {
		points[i] = domain.TrailPoint{QuoteID: ids[i], Position: positions[i]}
	}
	return points
}

func (r *ShelfRepository) TopCategories(ctx context.Context, userID string, limit int) ([]domain.Category, error) {
	rows, err := r.db.Query(ctx, `
		select subject, count(*) from (
			select distinct li.work_id, unnest(w.subjects) as subject
			from library_items li join works w on w.id = li.work_id
			where li.user_id = $1
		) s group by subject order by count(*) desc, subject limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := []domain.Category{}
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.Name, &c.Books); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}
