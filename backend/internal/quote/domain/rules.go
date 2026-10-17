// Package domain holds the rules for quotes.
//
// A quote is a short, cited excerpt from a reader's own copy of a work plus
// the reader's own reaction to it. The reaction is the point: a quote can only
// be made public, or discussed with a character, once the reader has written
// one — so what is published is the reader's perspective, not a copy of the
// work.
package domain

import (
	"strconv"
	"strings"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
)

// MaxWords caps the length of a single quote.
const MaxWords = 300

var (
	ErrEmpty            = kernel.Invalid("quote text is empty")
	ErrTooLong          = kernel.Invalid("quote is longer than " + strconv.Itoa(MaxWords) + " words")
	ErrReactionRequired = kernel.Invalid("write your reaction first")
	ErrCitationMissing  = kernel.Invalid("a public quote needs a location (page, chapter or position)")
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

// HasReaction reports whether the reader has written a reaction.
func HasReaction(reaction string) bool {
	return strings.TrimSpace(reaction) != ""
}

// ValidatePublic checks that a quote may be made public.
func ValidatePublic(text, reaction, location string) error {
	if err := ValidateText(text); err != nil {
		return err
	}
	if !HasReaction(reaction) {
		return ErrReactionRequired
	}
	if strings.TrimSpace(location) == "" {
		return ErrCitationMissing
	}
	return nil
}
