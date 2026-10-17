// Package postgres implements the catalog's repositories on Postgres.
package postgres

import (
	"context"
	"errors"

	"github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernelpg "github.com/an4eetos/serendipity/backend/internal/kernel/adapters/postgres"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// WorkColumns selects a work aliased "w"; scan it with WorkDest. Other
// contexts' repositories use them to join works.
const WorkColumns = `w.id, w.title, w.authors, w.isbn, w.openlibrary_key, w.cover_url, w.subjects`

// WorkDest returns the scan destinations matching WorkColumns.
func WorkDest(w *domain.Work) []any {
	return []any{&w.ID, &w.Title, &w.Authors, &w.ISBN, &w.OpenLibraryKey, &w.CoverURL, &w.Subjects}
}

// WorkRepository implements ports.WorkRepository.
type WorkRepository struct {
	db *kernelpg.DB
}

func NewWorkRepository(db *kernelpg.DB) *WorkRepository {
	return &WorkRepository{db: db}
}

func (r *WorkRepository) Ensure(ctx context.Context, w domain.Work, createdBy string) (domain.Work, error) {
	if w.Authors == nil {
		w.Authors = []string{}
	}
	if w.Subjects == nil {
		w.Subjects = []string{}
	}
	if w.OpenLibraryKey != nil {
		var out domain.Work
		err := r.db.QueryRow(ctx, `select `+WorkColumns+` from works w where w.openlibrary_key = $1`, *w.OpenLibraryKey).
			Scan(WorkDest(&out)...)
		if !errors.Is(err, kernel.ErrNotFound) {
			return out, err
		}
	}
	var out domain.Work
	err := r.db.QueryRow(ctx, `
		insert into works as w (title, authors, isbn, openlibrary_key, cover_url, subjects, created_by)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (openlibrary_key) do update set openlibrary_key = excluded.openlibrary_key
		returning `+WorkColumns,
		w.Title, w.Authors, w.ISBN, w.OpenLibraryKey, w.CoverURL, w.Subjects, createdBy).
		Scan(WorkDest(&out)...)
	return out, err
}
