package domain

import (
	discussion "github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// ErrOwnContent is returned when readers report their own content.
var ErrOwnContent = kernel.Invalid("you can't report your own content")

// Target is reportable content as it is now.
type Target struct {
	Ref     TargetRef
	OwnerID string
	// Public reports whether anyone can see the target now.
	Public  bool
	Removed bool
	// Quote is the quote, or the conversation's quote.
	Quote *quote.Quote
	// Conversation is nil for quotes.
	Conversation *discussion.Conversation
}

// QuoteTarget is a quote as reportable content.
func QuoteTarget(q quote.Quote) Target {
	return Target{
		Ref:     TargetRef{Kind: TargetQuote, ID: q.ID},
		OwnerID: q.UserID,
		Public:  q.IsPublic(),
		Removed: q.Removed(),
		Quote:   &q,
	}
}

// ConversationTarget is a discussion of q as reportable content. Public pages
// show a discussion only when its quote is public, it isn't removed and the
// character has answered.
func ConversationTarget(c discussion.Conversation, q quote.Quote) Target {
	return Target{
		Ref:          TargetRef{Kind: TargetConversation, ID: c.ID},
		OwnerID:      c.UserID,
		Public:       q.IsPublic() && !c.Removed() && c.Answered(),
		Removed:      c.Removed(),
		Quote:        &q,
		Conversation: &c,
	}
}

// CheckReportable reports whether reporterID may report t: readers report
// only what they can see, and never their own content.
func (t Target) CheckReportable(reporterID string) error {
	if !t.Public {
		return kernel.ErrNotFound
	}
	if reporterID == t.OwnerID {
		return ErrOwnContent
	}
	return nil
}
