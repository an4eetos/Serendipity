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

// PublicBook is a book as a profile shows it. Uploaded files are never
// shown.
type PublicBook struct {
	Work     catalog.Work
	Status   library.ReadingStatus
	Progress float64
	// PublicQuotes counts the quotes from this book shown on the profile.
	PublicQuotes int
}

// PublicProfile is what anyone may see of a profile.
type PublicProfile struct {
	Profile    Profile
	Categories []Category
	Books      []PublicBook
	Quotes     []discussion.Thread
}

// NewPublicProfile assembles a public profile, attaching each discussion to
// its quote. Discussions that never got a reply (e.g. a failed request) are
// left out.
func NewPublicProfile(p Profile, categories []Category, books []PublicBook, quotes []quote.Quote, convs []discussion.Conversation) PublicProfile {
	out := PublicProfile{Profile: p, Categories: categories, Books: books, Quotes: make([]discussion.Thread, len(quotes))}
	byQuote := make(map[string]*discussion.Thread, len(quotes))
	for i, q := range quotes {
		out.Quotes[i] = discussion.Thread{Quote: q, Conversations: []discussion.Conversation{}}
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
