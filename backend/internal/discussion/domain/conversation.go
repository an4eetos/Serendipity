package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

// MaxMessageLength caps a reader's message, in characters.
const MaxMessageLength = 4000

var (
	ErrMessageLength = kernel.Invalid("message must be 1–" + strconv.Itoa(MaxMessageLength) + " characters")
	ErrNotYourTurn   = kernel.Conflict("wait for the character to answer first")

	ErrRefused              = kernel.Invalid("the character declined to respond to this")
	ErrCharacterUnavailable = kernel.Unavailable("the character could not answer right now — try again")
	ErrReplyNotSaved        = kernel.Unavailable("could not save the reply")
)

// Persona is a character readers discuss their reactions with.
type Persona struct {
	ID       string
	Slug     string
	Name     string
	Era      string
	ShortBio string
	// SystemPrompt describes the character to the model. It is never shown
	// to readers.
	SystemPrompt string
	IsPreset     bool
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	ID        int64
	Role      Role
	Content   string
	CreatedAt time.Time
}

// Conversation is a discussion of the reader's reaction to a quote with one
// character.
type Conversation struct {
	ID       string
	UserID   string // the reader who owns the conversation
	QuoteID  string
	Persona  Persona
	Messages []Message
	// RemovedAt is when a moderator removed the conversation; nil unless
	// removed.
	RemovedAt *time.Time
	// RemovedReason tells the owner why the conversation was removed.
	RemovedReason string
	CreatedAt     time.Time
}

// Removed reports whether a moderator removed the conversation.
func (c Conversation) Removed() bool {
	return c.RemovedAt != nil
}

// Answered reports whether the character has replied at least once.
func (c Conversation) Answered() bool {
	return len(c.Messages) > 0
}

// CheckReaderTurn reports whether the reader may send the next message: the
// character must have answered the last one.
func (c Conversation) CheckReaderTurn() error {
	if n := len(c.Messages); n == 0 || c.Messages[n-1].Role != RoleAssistant {
		return ErrNotYourTurn
	}
	return nil
}

// NewReaderMessage trims and checks a message from the reader.
func NewReaderMessage(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > MaxMessageLength {
		return "", ErrMessageLength
	}
	return content, nil
}

// CheckDiscussable reports whether a quote may be discussed. The character
// discusses the reader's reaction rather than reacting to the passage on its
// own, so there must be one.
func CheckDiscussable(q quote.Quote) error {
	if !quote.HasReaction(q.Reaction) {
		return quote.ErrReactionRequired
	}
	return nil
}

// Thread is a quote with the discussions about it.
type Thread struct {
	Quote         quote.Quote
	Conversations []Conversation
}
