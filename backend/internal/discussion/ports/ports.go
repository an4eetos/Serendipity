// Package ports declares what the discussion context needs from the outside.
package ports

import (
	"context"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
)

// PersonaRepository stores characters.
type PersonaRepository interface {
	// List returns the presets, then the reader's own active characters (not
	// archived, not removed): the characters a reader can start a discussion with.
	List(ctx context.Context, userID string) ([]domain.Persona, error)
	// Get returns a preset or one of the reader's own characters, in any state.
	Get(ctx context.Context, userID, id string) (domain.Persona, error)
}

// CharacterRepository stores readers' own characters. Every method is scoped
// to ownerID; presets and other readers' characters are kernel.ErrNotFound.
type CharacterRepository interface {
	// ListOwn returns every custom character of ownerID, newest first, in any
	// state, with HasConversations.
	ListOwn(ctx context.Context, ownerID string) ([]domain.Persona, error)
	// GetOwn returns one, with HasConversations. Call it before a
	// transaction: the id comes from the reader.
	GetOwn(ctx context.Context, ownerID, id string) (domain.Persona, error)
	// GetOwnForUpdate locks the row (FOR NO KEY UPDATE) and then reads
	// HasConversations in a second statement. It must run inside the TxManager.
	GetOwnForUpdate(ctx context.Context, ownerID, id string) (domain.Persona, error)
	// LockOwner serializes changes to ownerID's active characters until the
	// transaction ends. It must run inside the TxManager.
	LockOwner(ctx context.Context, ownerID string) error
	// CountActive counts ownerID's characters that are neither archived nor removed.
	CountActive(ctx context.Context, ownerID string) (int, error)
	// Create stores a validated draft. The slug is slugBase plus a random suffix.
	Create(ctx context.Context, ownerID string, d domain.CharacterDraft, slugBase string) (domain.Persona, error)
	// Update writes name, era, short_bio, system_prompt and archived_at of a
	// character that isn't removed, and returns it re-read. A rename of a used
	// character returns domain.ErrNameLocked.
	Update(ctx context.Context, p domain.Persona) (domain.Persona, error)
}

// PersonaModeration is how moderators act on any reader's custom characters;
// it is not scoped to a reader.
type PersonaModeration interface {
	// ListAny returns these custom characters, prompt included, whoever owns
	// them; missing ids and presets are left out.
	ListAny(ctx context.Context, ids []string) ([]domain.ModeratedPersona, error)
	// Remove sets removed_at/removed_reason. It returns kernel.ErrNotFound
	// unless the character exists, is custom and isn't removed.
	Remove(ctx context.Context, id, reason string) error
	// Restore clears them. It returns kernel.ErrNotFound unless it is removed.
	Restore(ctx context.Context, id string) error
}

// ConversationRepository stores discussions. Every method that takes a user
// id is scoped to the reader's own conversations.
type ConversationRepository interface {
	// Create returns domain.ErrCharacterArchived or domain.ErrCharacterRemoved
	// when the character isn't usable.
	Create(ctx context.Context, userID, quoteID, personaID string) (string, error)
	Delete(ctx context.Context, userID, id string) error
	// Get returns a conversation with its persona and messages.
	Get(ctx context.Context, userID, id string) (domain.Conversation, error)
	ListForQuote(ctx context.Context, userID, quoteID string) ([]domain.Conversation, error)
	// ListForQuotes lists the discussions public pages show for these quotes:
	// removed ones and those whose character is removed are left out.
	ListForQuotes(ctx context.Context, userID string, quoteIDs []string) ([]domain.Conversation, error)
	// AppendMessage returns domain.ErrCharacterArchived or
	// domain.ErrCharacterRemoved when the character isn't usable, and
	// domain.ErrConversationRemoved when the conversation is removed.
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
