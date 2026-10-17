// Package domain holds the catalog's rules: which book metadata Serendipity
// accepts. A work is catalog metadata only, never the text itself.
package domain

import (
	"strings"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

const (
	// MaxTitleLength caps a work's title, in bytes.
	MaxTitleLength = 500
	// MaxSubjects caps how many subjects a work keeps.
	MaxSubjects = 8

	openLibraryWorkPrefix = "/works/"
	// Covers are only shown from Open Library's cover host.
	trustedCoverPrefix = "https://covers.openlibrary.org/"
)

var (
	ErrTitleRequired  = kernel.Invalid("title is required")
	ErrInvalidWorkKey = kernel.Invalid("invalid Open Library key")
)

// Work is a book's catalog metadata.
type Work struct {
	ID             string
	Title          string
	Authors        []string
	ISBN           *string
	OpenLibraryKey *string
	CoverURL       *string
	Subjects       []string
}

// NewWork checks the metadata of a work that is not stored yet. Untrusted
// covers are dropped and subjects capped rather than rejected.
func NewWork(w Work) (Work, error) {
	w.Title = strings.TrimSpace(w.Title)
	if w.Title == "" || len(w.Title) > MaxTitleLength {
		return Work{}, ErrTitleRequired
	}
	if w.OpenLibraryKey != nil && !strings.HasPrefix(*w.OpenLibraryKey, openLibraryWorkPrefix) {
		return Work{}, ErrInvalidWorkKey
	}
	if w.CoverURL != nil && !strings.HasPrefix(*w.CoverURL, trustedCoverPrefix) {
		w.CoverURL = nil
	}
	w.Subjects = CapSubjects(w.Subjects)
	return w, nil
}

// CapSubjects keeps the first MaxSubjects subjects.
func CapSubjects(subjects []string) []string {
	if len(subjects) > MaxSubjects {
		return subjects[:MaxSubjects]
	}
	return subjects
}
