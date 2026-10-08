// Package quote holds the copyright guardrails for quotes.
//
// A quote is a short, cited excerpt from a reader's own copy of a work. It can
// only be made public when the reader adds their own note, so what is
// published is commentary on the work rather than a copy of it.
package quote

import (
	"errors"
	"fmt"
	"strings"
)

// MaxWords caps the length of a single quote.
const MaxWords = 300

var (
	ErrEmpty           = errors.New("quote text is empty")
	ErrTooLong         = fmt.Errorf("quote is longer than %d words", MaxWords)
	ErrNoteRequired    = errors.New("add your own note before making a quote public")
	ErrCitationMissing = errors.New("a public quote needs a location (page, chapter or position)")
)

// WordCount counts whitespace-separated words.
func WordCount(s string) int {
	return len(strings.Fields(s))
}

// ValidateText checks a quote's text against the length cap.
func ValidateText(text string) error {
	n := WordCount(text)
	if n == 0 {
		return ErrEmpty
	}
	if n > MaxWords {
		return ErrTooLong
	}
	return nil
}

// ValidatePublic checks that a quote may be made public.
func ValidatePublic(text, note, location string) error {
	if err := ValidateText(text); err != nil {
		return err
	}
	if strings.TrimSpace(note) == "" {
		return ErrNoteRequired
	}
	if strings.TrimSpace(location) == "" {
		return ErrCitationMissing
	}
	return nil
}
