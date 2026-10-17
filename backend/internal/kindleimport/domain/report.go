package domain

import quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"

// Report sums up an import.
type Report struct {
	Books          int
	Imported       int
	Duplicates     int
	SkippedTooLong int
	OrphanNotes    int
}

// Draft is the quote a highlight becomes in the reader's library item.
func (h Highlight) Draft(workID, libraryItemID string) quote.Draft {
	return quote.Draft{
		WorkID:        workID,
		LibraryItemID: &libraryItemID,
		Text:          h.Text,
		Location:      h.Location(),
		// A Kindle note is the reader's reaction to that highlight.
		Reaction: h.Note,
		Source:   quote.SourceKindle,
	}
}

// Authors lists the book's author, if Kindle named one.
func (b Book) Authors() []string {
	if b.Author == "" {
		return nil
	}
	return []string{b.Author}
}
