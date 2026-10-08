package store

import (
	"context"
	"strings"
)

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

const profileCols = `id, username, display_name, bio, is_public, created_at`

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	err := row.Scan(&p.ID, &p.Username, &p.DisplayName, &p.Bio, &p.IsPublic, &p.CreatedAt)
	return p, notFound(err)
}

func (s *Store) GetProfile(ctx context.Context, userID string) (Profile, error) {
	return scanProfile(s.pool.QueryRow(ctx, `select `+profileCols+` from profiles where id = $1`, userID))
}

func (s *Store) GetProfileByUsername(ctx context.Context, username string) (Profile, error) {
	return scanProfile(s.pool.QueryRow(ctx, `select `+profileCols+` from profiles where username = $1`, strings.ToLower(username)))
}

type ProfilePatch struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	IsPublic    *bool   `json:"is_public"`
}

func (s *Store) UpdateProfile(ctx context.Context, userID string, p ProfilePatch) (Profile, error) {
	return scanProfile(s.pool.QueryRow(ctx, `
		update profiles set
			username     = coalesce($2, username),
			display_name = coalesce($3, display_name),
			bio          = coalesce($4, bio),
			is_public    = coalesce($5, is_public)
		where id = $1
		returning `+profileCols,
		userID, p.Username, p.DisplayName, p.Bio, p.IsPublic))
}

// ---------------------------------------------------------------------------
// Works
// ---------------------------------------------------------------------------

const workCols = `w.id, w.title, w.authors, w.isbn, w.openlibrary_key, w.cover_url, w.subjects`

func scanWorkInto(w *Work) []any {
	return []any{&w.ID, &w.Title, &w.Authors, &w.ISBN, &w.OpenLibraryKey, &w.CoverURL, &w.Subjects}
}

// EnsureWork returns the work with w's Open Library key, creating it from w if
// it does not exist yet. Works without a key are always created.
func (s *Store) EnsureWork(ctx context.Context, w Work, createdBy string) (Work, error) {
	if w.Authors == nil {
		w.Authors = []string{}
	}
	if w.Subjects == nil {
		w.Subjects = []string{}
	}
	if w.OpenLibraryKey != nil {
		var out Work
		err := s.pool.QueryRow(ctx, `select `+workCols+` from works w where w.openlibrary_key = $1`, *w.OpenLibraryKey).
			Scan(scanWorkInto(&out)...)
		if err == nil {
			return out, nil
		}
		if err = notFound(err); err != ErrNotFound {
			return Work{}, err
		}
	}
	var out Work
	err := s.pool.QueryRow(ctx, `
		insert into works as w (title, authors, isbn, openlibrary_key, cover_url, subjects, created_by)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (openlibrary_key) do update set openlibrary_key = excluded.openlibrary_key
		returning `+workCols,
		w.Title, w.Authors, w.ISBN, w.OpenLibraryKey, w.CoverURL, w.Subjects, createdBy).
		Scan(scanWorkInto(&out)...)
	return out, err
}

// ---------------------------------------------------------------------------
// Library
// ---------------------------------------------------------------------------

const libraryCols = `li.id, ` + workCols + `, li.file_path, li.file_type, li.progress, li.status,
	li.last_location, (select count(*) from quotes q where q.library_item_id = li.id), li.created_at, li.updated_at`

const libraryFrom = ` from library_items li join works w on w.id = li.work_id `

func scanLibraryItem(row interface{ Scan(...any) error }) (LibraryItem, error) {
	var li LibraryItem
	dest := append([]any{&li.ID}, scanWorkInto(&li.Work)...)
	dest = append(dest, &li.FilePath, &li.FileType, &li.Progress, &li.Status,
		&li.LastLocation, &li.QuoteCount, &li.CreatedAt, &li.UpdatedAt)
	err := row.Scan(dest...)
	return li, notFound(err)
}

func (s *Store) AddToLibrary(ctx context.Context, userID, workID string) (LibraryItem, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		insert into library_items (user_id, work_id) values ($1, $2)
		on conflict (user_id, work_id) do update set updated_at = now()
		returning id`, userID, workID).Scan(&id)
	if err != nil {
		return LibraryItem{}, err
	}
	return s.GetLibraryItem(ctx, userID, id)
}

func (s *Store) GetLibraryItem(ctx context.Context, userID, id string) (LibraryItem, error) {
	return scanLibraryItem(s.pool.QueryRow(ctx,
		`select `+libraryCols+libraryFrom+`where li.user_id = $1 and li.id = $2`, userID, id))
}

func (s *Store) ListLibrary(ctx context.Context, userID string) ([]LibraryItem, error) {
	rows, err := s.pool.Query(ctx,
		`select `+libraryCols+libraryFrom+`where li.user_id = $1 order by li.updated_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LibraryItem{}
	for rows.Next() {
		li, err := scanLibraryItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

// FindLibraryItemByTitle matches a reader's book by case-insensitive title.
func (s *Store) FindLibraryItemByTitle(ctx context.Context, userID, title string) (LibraryItem, error) {
	return scanLibraryItem(s.pool.QueryRow(ctx,
		`select `+libraryCols+libraryFrom+`where li.user_id = $1 and lower(w.title) = lower($2)
		order by li.created_at limit 1`, userID, title))
}

type LibraryPatch struct {
	Progress     *float64 `json:"progress"`
	Status       *string  `json:"status"`
	LastLocation *string  `json:"last_location"`
	FilePath     *string  `json:"file_path"`
	FileType     *string  `json:"file_type"`
}

func (s *Store) UpdateLibraryItem(ctx context.Context, userID, id string, p LibraryPatch) (LibraryItem, error) {
	tag, err := s.pool.Exec(ctx, `
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
		return LibraryItem{}, err
	}
	if tag.RowsAffected() == 0 {
		return LibraryItem{}, ErrNotFound
	}
	return s.GetLibraryItem(ctx, userID, id)
}
