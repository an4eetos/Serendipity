package domain

import (
	"strings"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// MinQueryLength is the shortest catalog search query, in bytes.
const MinQueryLength = 2

var (
	ErrQueryTooShort     = kernel.Invalid("search query is too short")
	ErrSearchUnavailable = kernel.Unavailable("book search is unavailable right now")
)

// SearchResult is a catalog entry found by a search, not yet a work in
// Serendipity.
type SearchResult struct {
	OpenLibraryKey string
	Title          string
	Authors        []string
	ISBN           *string
	CoverURL       *string
	Subjects       []string
	FirstPublished *int
}

// SearchQuery trims q and checks that it is long enough to search for.
func SearchQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if len(q) < MinQueryLength {
		return "", ErrQueryTooShort
	}
	return q, nil
}
