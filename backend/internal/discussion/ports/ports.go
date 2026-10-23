// Package ports declares what the discussion context needs from the outside.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
)

// PersonaRepository stores characters.
type PersonaRepository interface {
	// List returns the presets plus the reader's own characters.
	List(ctx context.Context, userID string) ([]domain.Persona, error)
	// Get returns a preset or one of the reader's own characters.
	Get(ctx context.Context, userID, id string) (domain.Persona, error)
}

// ConversationRepository stores discussions. Every method that takes a user
// id is scoped to the reader's own conversations.
type ConversationRepository interface {
	Create(ctx context.Context, userID, quoteID, personaID string) (string, error)
	Delete(ctx context.Context, userID, id string) error
	// Get returns a conversation with its persona and messages.
	Get(ctx context.Context, userID, id string) (domain.Conversation, error)
	ListForQuote(ctx context.Context, userID, quoteID string) ([]domain.Conversation, error)
	// ListForQuotes lists the discussions public pages show for these quotes:
	// removed ones are left out.
	ListForQuotes(ctx context.Context, userID string, quoteIDs []string) ([]domain.Conversation, error)
	AppendMessage(ctx context.Context, conversationID string, role domain.Role, content string) (domain.Message, error)
	DeleteMessage(ctx context.Context, conversationID string, id int64) error
}

// ConversationModeration is how moderators act on any reader's discussions;
// it is not scoped to a reader.
type ConversationModeration interface {
	// ListAny returns these conversations with persona and messages, removed
	// ones included; missing ids are left out.
	ListAny(ctx context.Context, ids []string) ([]domain.Conversation, error)
	// Remove sets removed_at/removed_reason. It returns kernel.ErrNotFound
	// unless the conversation exists and isn't removed.
	Remove(ctx context.Context, id, reason string) error
	// Restore clears them. It returns kernel.ErrNotFound unless the
	// conversation exists and is removed.
	Restore(ctx context.Context, id string) error
}

// CharacterModel generates characters' replies.
type CharacterModel interface {
	// Reply generates the character's next reply, calling onText with each
	// chunk as it arrives, and returns the full reply. It returns
	// domain.ErrRefused when the model declines to answer; chunks already
	// passed to onText must then be discarded.
	Reply(ctx context.Context, p domain.CharacterPrompt, onText func(string) error) (string, error)
}

// ReplyStream receives a character's reply as it is generated. Driving
// adapters implement it to forward the reply to the reader.
type ReplyStream interface {
	// Started is called once the conversation exists, before the reply is
	// generated.
	Started(conversationID string) error
	Text(chunk string) error
}
