package usecase

import (
	"context"
	"strings"

	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/ports"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
	quoteports "github.com/an4eetos/serendipity/backend/internal/quote/ports"
)

type fakeQuotes struct {
	quoteports.QuoteRepository
	quotes map[string]quote.Quote
}

func (f *fakeQuotes) Get(_ context.Context, _, id string) (quote.Quote, error) {
	q, ok := f.quotes[id]
	if !ok {
		return quote.Quote{}, kernel.ErrNotFound
	}
	return q, nil
}

type fakePersonas struct {
	ports.PersonaRepository
	persona domain.Persona
}

func (f *fakePersonas) Get(_ context.Context, _, id string) (domain.Persona, error) {
	if id != f.persona.ID {
		return domain.Persona{}, kernel.ErrNotFound
	}
	return f.persona, nil
}

// fakeConversations records writes, and every write made outside a unit of
// work.
type fakeConversations struct {
	ports.ConversationRepository
	conv domain.Conversation
	list []domain.Conversation

	created         []string
	deleted         []string
	appended        []domain.Message
	deletedMessages []int64
	outsideTx       []string

	appendAssistantErr error
	deleteErr          error
}

func (f *fakeConversations) write(ctx context.Context, op string) {
	if !portstest.InTx(ctx) {
		f.outsideTx = append(f.outsideTx, op)
	}
}

func (f *fakeConversations) Create(ctx context.Context, _, quoteID, _ string) (string, error) {
	f.write(ctx, "create")
	id := "conv-" + quoteID
	f.created = append(f.created, id)
	return id, nil
}

func (f *fakeConversations) Delete(ctx context.Context, _, id string) error {
	f.write(ctx, "delete")
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeConversations) Get(_ context.Context, _, id string) (domain.Conversation, error) {
	if id != f.conv.ID {
		return domain.Conversation{}, kernel.ErrNotFound
	}
	return f.conv, nil
}

func (f *fakeConversations) ListForQuote(context.Context, string, string) ([]domain.Conversation, error) {
	return f.list, nil
}

func (f *fakeConversations) AppendMessage(ctx context.Context, _ string, role domain.Role, content string) (domain.Message, error) {
	f.write(ctx, "append")
	if role == domain.RoleAssistant && f.appendAssistantErr != nil {
		return domain.Message{}, f.appendAssistantErr
	}
	m := domain.Message{ID: int64(100 + len(f.appended)), Role: role, Content: content}
	f.appended = append(f.appended, m)
	return m, nil
}

func (f *fakeConversations) DeleteMessage(ctx context.Context, _ string, id int64) error {
	f.write(ctx, "delete message")
	f.deletedMessages = append(f.deletedMessages, id)
	return f.deleteErr
}

type fakeModel struct {
	chunks []string
	err    error
	prompt domain.CharacterPrompt
	calls  int
}

func (m *fakeModel) Reply(_ context.Context, p domain.CharacterPrompt, onText func(string) error) (string, error) {
	m.calls++
	m.prompt = p
	for _, c := range m.chunks {
		if err := onText(c); err != nil {
			return "", err
		}
	}
	if m.err != nil {
		return "", m.err
	}
	return strings.Join(m.chunks, ""), nil
}

type fakeStream struct {
	started string
	text    []string
}

func (s *fakeStream) Started(id string) error {
	s.started = id
	return nil
}

func (s *fakeStream) Text(chunk string) error {
	s.text = append(s.text, chunk)
	return nil
}
