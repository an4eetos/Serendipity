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
	// Quote is the quote, or the conversation's quote; nil for characters.
	Quote *quote.Quote
	// Conversation is nil for quotes and characters.
	Conversation *discussion.Conversation
	// Character is set for characters only.
	Character *discussion.Persona
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
// show a discussion only when its quote is public, neither it nor its
// character is removed, and the character has answered.
func ConversationTarget(c discussion.Conversation, q quote.Quote) Target {
	return Target{
		Ref:          TargetRef{Kind: TargetConversation, ID: c.ID},
		OwnerID:      c.UserID,
		Public:       q.IsPublic() && !c.Removed() && !c.Persona.Removed() && c.Answered(),
		Removed:      c.Removed(),
		Quote:        &q,
		Conversation: &c,
	}
}

// CharacterTarget is a reader's own character as reportable content. Readers
// see it only next to a discussion that public pages show.
func CharacterTarget(m discussion.ModeratedPersona) Target {
	p := m.Persona
	return Target{
		Ref:       TargetRef{Kind: TargetCharacter, ID: p.ID},
		OwnerID:   p.OwnerID,
		Public:    m.InPublicDiscussion && !p.Removed(),
		Removed:   p.Removed(),
		Character: &p,
	}
}

// OnProfile returns t as seen by readers, given whether its owner's profile
// is public: content on a private profile is on no public page, however its
// own visibility is set.
func (t Target) OnProfile(profilePublic bool) Target {
	t.Public = t.Public && profilePublic
	return t
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
