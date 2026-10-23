// Package converters maps discussion HTTP requests, responses and stream
// events to domain values. Other contexts use ConversationResponse to embed
// discussions in theirs.
package converters

import (
	"errors"
	"time"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	quoteconv "github.com/an4eetos/serendipity/backend/internal/quote/adapters/converters"
)

type StartRequest struct {
	QuoteID   string `json:"quote_id"`
	PersonaID string `json:"persona_id"`
}

type PostMessageRequest struct {
	Content string `json:"content"`
}

type PersonaResponse struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Era      string `json:"era"`
	ShortBio string `json:"short_bio"`
	IsPreset bool   `json:"is_preset"`
}

// ToPersonaResponse leaves out the system prompt, which readers never see.
func ToPersonaResponse(p domain.Persona) PersonaResponse {
	return PersonaResponse{ID: p.ID, Slug: p.Slug, Name: p.Name, Era: p.Era, ShortBio: p.ShortBio, IsPreset: p.IsPreset}
}

func ToPersonaResponses(ps []domain.Persona) []PersonaResponse {
	out := make([]PersonaResponse, len(ps))
	for i, p := range ps {
		out[i] = ToPersonaResponse(p)
	}
	return out
}

type MessageResponse struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func ToMessageResponse(m domain.Message) MessageResponse {
	return MessageResponse{ID: m.ID, Role: string(m.Role), Content: m.Content, CreatedAt: m.CreatedAt}
}

type ConversationResponse struct {
	ID            string            `json:"id"`
	QuoteID       string            `json:"quote_id"`
	Persona       PersonaResponse   `json:"persona"`
	Messages      []MessageResponse `json:"messages"`
	RemovedAt     *time.Time        `json:"removed_at"`
	RemovedReason string            `json:"removed_reason"`
	CreatedAt     time.Time         `json:"created_at"`
}

func ToConversationResponses(cs []domain.Conversation) []ConversationResponse {
	out := make([]ConversationResponse, len(cs))
	for i, c := range cs {
		msgs := make([]MessageResponse, len(c.Messages))
		for j, m := range c.Messages {
			msgs[j] = ToMessageResponse(m)
		}
		out[i] = ConversationResponse{
			ID:            c.ID,
			QuoteID:       c.QuoteID,
			Persona:       ToPersonaResponse(c.Persona),
			Messages:      msgs,
			RemovedAt:     c.RemovedAt,
			RemovedReason: c.RemovedReason,
			CreatedAt:     c.CreatedAt,
		}
	}
	return out
}

// ThreadResponse is a quote's fields plus its discussions.
type ThreadResponse struct {
	quoteconv.QuoteResponse
	Conversations []ConversationResponse `json:"conversations"`
}

func ToThreadResponse(t domain.Thread) ThreadResponse {
	return ThreadResponse{
		QuoteResponse: quoteconv.ToQuoteResponse(t.Quote),
		Conversations: ToConversationResponses(t.Conversations),
	}
}

// Event is one server-sent event of a streamed reply:
//
//	data: {"type":"start","conversation_id":"…"}
//	data: {"type":"delta","text":"…"}            (repeated)
//	data: {"type":"done","message":{…}}
//	data: {"type":"error","error":"…"}          (instead of done)
type Event struct {
	Type           string           `json:"type"`
	ConversationID string           `json:"conversation_id,omitempty"`
	Text           string           `json:"text,omitempty"`
	Message        *MessageResponse `json:"message,omitempty"`
	Error          string           `json:"error,omitempty"`
}

func StartEvent(conversationID string) Event {
	return Event{Type: "start", ConversationID: conversationID}
}

func DeltaEvent(text string) Event {
	return Event{Type: "delta", Text: text}
}

func DoneEvent(m domain.Message) Event {
	resp := ToMessageResponse(m)
	return Event{Type: "done", Message: &resp}
}

// ErrorEvent tells the reader why the reply failed, without internals.
func ErrorEvent(err error) Event {
	msg := domain.ErrCharacterUnavailable.Error()
	var de kernel.DomainError
	if errors.As(err, &de) {
		msg = de.Error()
	}
	return Event{Type: "error", Error: msg}
}
