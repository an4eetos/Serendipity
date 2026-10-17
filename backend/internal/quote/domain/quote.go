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
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Work       catalog.Work
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
}

// CheckPatch reports whether p may be applied to q: the patched quote must
// still meet the publishing rules.
func (q Quote) CheckPatch(p Patch) error {
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
