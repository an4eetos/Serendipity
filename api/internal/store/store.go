// Package store is the Postgres data access layer.
//
// The API connects with a privileged role that bypasses row-level security,
// so every query that touches user data filters by the caller's user id.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

type Profile struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

type Work struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	ISBN           *string  `json:"isbn"`
	OpenLibraryKey *string  `json:"openlibrary_key"`
	CoverURL       *string  `json:"cover_url"`
	Subjects       []string `json:"subjects"`
}

type LibraryItem struct {
	ID           string    `json:"id"`
	Work         Work      `json:"work"`
	FilePath     *string   `json:"file_path"`
	FileType     *string   `json:"file_type"`
	Progress     float64   `json:"progress"`
	Status       string    `json:"status"`
	LastLocation *string   `json:"last_location"`
	QuoteCount   int       `json:"quote_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Quote struct {
	ID            string    `json:"id"`
	WorkID        string    `json:"work_id"`
	LibraryItemID *string   `json:"library_item_id"`
	Text          string    `json:"text"`
	Location      string    `json:"location"`
	Anchor        *string   `json:"anchor"`
	Position      *float64  `json:"position"`
	Note          string    `json:"note"`
	Source        string    `json:"source"`
	Visibility    string    `json:"visibility"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Work          *Work     `json:"work,omitempty"`
}

type Persona struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Era          string `json:"era"`
	ShortBio     string `json:"short_bio"`
	SystemPrompt string `json:"-"`
	IsPreset     bool   `json:"is_preset"`
}

type Conversation struct {
	ID        string    `json:"id"`
	QuoteID   string    `json:"quote_id"`
	Mode      string    `json:"mode"`
	Persona   Persona   `json:"persona"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
