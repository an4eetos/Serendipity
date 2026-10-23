package domain

import (
	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	library "github.com/an4eetos/serendipity/backend/internal/library/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

const (
	// ShownQuotes caps how many of the newest public quotes a profile shows.
	ShownQuotes = 100
	// ShownCategories caps how many categories a profile shows.
	ShownCategories = 12
)

// Category is a subject of the books on a profile.
type Category struct {
	Name  string
	Books int
}

// TrailPoint is a listed quote placed on a book's reading trail.
type TrailPoint struct {
	QuoteID string
	// Position is how far into the book the quote is, from 0 to 1.
	Position float64
}

// PublicBook is a book as a profile shows it. Uploaded files are never
// shown.
type PublicBook struct {
	Work     catalog.Work
	Status   library.ReadingStatus
	Progress float64
	// PublicQuotes counts the book's listed quotes (public, not removed),
	// collapsed ones included.
	PublicQuotes int
	// Trail holds the listed quotes that have a position, by position
	// ascending. It isn't capped by ShownQuotes.
	Trail []TrailPoint
}

// Unplaced counts the listed quotes with no position on the trail, such as
// imported Kindle highlights.
func (b PublicBook) Unplaced() int {
	return b.PublicQuotes - len(b.Trail)
}

// PublicProfile is what anyone may see of a profile.
type PublicProfile struct {
	Profile    Profile
	Categories []Category
	Books      []PublicBook
	Quotes     []discussion.Thread
}

// NewPublicProfile assembles a public profile, attaching each discussion to
// its quote. Every quote is shown as public pages show it (a collapsed
// passage is blanked). Discussions that never got a reply (e.g. a failed
// request) are left out.
func NewPublicProfile(p Profile, categories []Category, books []PublicBook, quotes []quote.Quote, convs []discussion.Conversation) PublicProfile {
	out := PublicProfile{Profile: p, Categories: categories, Books: books, Quotes: make([]discussion.Thread, len(quotes))}
	byQuote := make(map[string]*discussion.Thread, len(quotes))
	for i, q := range quotes {
		out.Quotes[i] = discussion.Thread{Quote: q.ForPublic(), Conversations: []discussion.Conversation{}}
		byQuote[q.ID] = &out.Quotes[i]
	}
	for _, c := range convs {
		if !c.Answered() {
			continue
		}
		if t := byQuote[c.QuoteID]; t != nil {
			t.Conversations = append(t.Conversations, c)
		}
	}
	return out
}
