package domain

import (
	"strings"
	"time"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

const (
	// MaxReactionLength caps a reaction, in bytes.
	MaxReactionLength = 5000
	// MaxLocationLength caps a citation, in characters.
	MaxLocationLength = 200
)

var (
	ErrReactionTooLong   = kernel.Invalid("reaction is too long")
	ErrInvalidVisibility = kernel.Invalid("invalid visibility")
	ErrRemoved           = kernel.Conflict("a moderator removed this quote, so it can't be made public")
)

// Source is where a quote came from.
type Source string

const (
	SourceUpload Source = "upload"
	SourceKindle Source = "kindle"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

func (v Visibility) Valid() bool {
	return v == VisibilityPrivate || v == VisibilityPublic
}

type Quote struct {
	ID            string
	UserID        string // the reader who owns the quote
	WorkID        string
	LibraryItemID *string
	Text          string
	// Location is the human-readable citation, e.g. "Page 12 · Loc 170-172".
	Location string
	// Anchor jumps back into the reader (EPUB CFI or "page:N").
	Anchor *string
	// Position is how far into the book the quote is, from 0 to 1.
	Position   *float64
	Reaction   string
	Source     Source
	Visibility Visibility
	// PassageHidden collapses the passage on public pages: the work's public
	// coverage cap was reached when the quote was published.
	PassageHidden bool
	// PassageWords is the passage's length as coverage counts it. The
	// database computes it.
	PassageWords int
	// RemovedAt is when a moderator removed the quote; nil unless removed.
	RemovedAt *time.Time
	// RemovedReason tells the owner why the quote was removed.
	RemovedReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Work          catalog.Work
}

// Removed reports whether a moderator removed the quote.
func (q Quote) Removed() bool {
	return q.RemovedAt != nil
}

// IsPublic reports whether anyone may see the quote: it is public and not
// removed.
func (q Quote) IsPublic() bool {
	return q.Visibility == VisibilityPublic && !q.Removed()
}

// Publishes reports whether p makes the quote public when it isn't yet.
func (q Quote) Publishes(p Patch) bool {
	return p.Visibility != nil && *p.Visibility == VisibilityPublic && q.Visibility != VisibilityPublic
}

// ForPublic is the quote as public pages show it: a collapsed passage is
// blanked and the removal details, which only the owner sees, are left out.
func (q Quote) ForPublic() Quote {
	if q.PassageHidden {
		q.Text = ""
	}
	q.RemovedAt = nil
	q.RemovedReason = ""
	return q
}

// Draft is a quote that is not stored yet.
type Draft struct {
	WorkID        string
	LibraryItemID *string
	Text          string
	Location      string
	Anchor        *string
	Position      *float64
	Reaction      string
	Source        Source
}

// NewUpload checks a quote the reader took from their own copy of a work. A
// position outside the book is dropped and an overlong citation cut rather
// than rejected.
func NewUpload(d Draft) (Draft, error) {
	d.Text = strings.TrimSpace(d.Text)
	if err := ValidateText(d.Text); err != nil {
		return Draft{}, err
	}
	if len(d.Reaction) > MaxReactionLength {
		return Draft{}, ErrReactionTooLong
	}
	if d.Position != nil && (*d.Position < 0 || *d.Position > 1) {
		d.Position = nil
	}
	d.Location = truncate(strings.TrimSpace(d.Location), MaxLocationLength)
	d.Source = SourceUpload
	return d, nil
}

// Patch changes a quote; nil fields stay as they are.
type Patch struct {
	Reaction   *string
	Visibility *Visibility
	// PassageHidden is decided by the publishing rules in UpdateQuote, never by
	// the reader: converters must not set it.
	PassageHidden *bool
}

// CheckPatch reports whether p may be applied to q: a removed quote can't be
// made public, and the patched quote must still meet the publishing rules.
func (q Quote) CheckPatch(p Patch) error {
	if p.Visibility != nil && *p.Visibility == VisibilityPublic && q.Removed() {
		return ErrRemoved
	}
	if p.Visibility != nil && !p.Visibility.Valid() {
		return ErrInvalidVisibility
	}
	if p.Reaction != nil && len(*p.Reaction) > MaxReactionLength {
		return ErrReactionTooLong
	}
	reaction, visibility := q.Reaction, q.Visibility
	if p.Reaction != nil {
		reaction = *p.Reaction
	}
	if p.Visibility != nil {
		visibility = *p.Visibility
	}
	if visibility == VisibilityPublic {
		return ValidatePublic(q.Text, reaction, q.Location)
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
