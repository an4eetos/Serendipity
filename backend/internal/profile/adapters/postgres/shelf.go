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

func (r *ShelfRepository) Books(ctx context.Context, userID string) ([]domain.PublicBook, error) {
	rows, err := r.db.Query(ctx, `
		select `+catalogpg.WorkColumns+`, li.status, li.progress,
			(select count(*) from quotes q where q.user_id = li.user_id and q.work_id = li.work_id and q.visibility = 'public')
		from library_items li join works w on w.id = li.work_id
		where li.user_id = $1 order by li.updated_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	books := []domain.PublicBook{}
	for rows.Next() {
		var b domain.PublicBook
		dest := append(catalogpg.WorkDest(&b.Work), &b.Status, &b.Progress, &b.PublicQuotes)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
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
