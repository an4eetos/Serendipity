package usecase

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	catalog "github.com/an4eetos/serendipity/backend/internal/catalog/domain"
	"github.com/an4eetos/serendipity/backend/internal/discussion/domain"
	kernel "github.com/an4eetos/serendipity/backend/internal/kernel/domain"
	"github.com/an4eetos/serendipity/backend/internal/kernel/ports/portstest"
	quote "github.com/an4eetos/serendipity/backend/internal/quote/domain"
)

var (
	diogenes  = domain.Persona{ID: "p1", Name: "Diogenes", SystemPrompt: "You are Diogenes."}
	reacted   = quote.Quote{ID: "q1", Text: "Know thyself.", Reaction: "Easier said.", Work: catalog.Work{Title: "Apology"}}
	unreacted = quote.Quote{ID: "q2", Text: "Know thyself."}
)

type startFixture struct {
	txm   *portstest.TxManager
	convs *fakeConversations
	model *fakeModel
	out   *fakeStream
	uc    *StartConversation
}

func newStart(model *fakeModel) startFixture {
	f := startFixture{
		txm:   &portstest.TxManager{},
		convs: &fakeConversations{},
		model: model,
		out:   &fakeStream{},
	}
	quotes := &fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted, unreacted.ID: unreacted}}
	f.uc = NewStartConversation(f.txm, quotes, &fakePersonas{persona: diogenes}, f.convs, model)
	return f
}

func TestStartConversation(t *testing.T) {
	t.Run("streams and stores the first reply", func(t *testing.T) {
		f := newStart(&fakeModel{chunks: []string{"Said by ", "a man in a barrel."}})
		m, err := f.uc.Execute(t.Context(), "u1", reacted.ID, diogenes.ID, f.out)
		if err != nil {
			t.Fatal(err)
		}
		if m.Role != domain.RoleAssistant || m.Content != "Said by a man in a barrel." {
			t.Errorf("reply = %#v", m)
		}
		if f.out.started != "conv-q1" || !reflect.DeepEqual(f.out.text, f.model.chunks) {
			t.Errorf("stream got start %q, text %q", f.out.started, f.out.text)
		}
		want := domain.NewCharacterPrompt(diogenes, domain.PassageOf(reacted), nil)
		if !reflect.DeepEqual(f.model.prompt, want) {
			t.Errorf("prompt = %#v, want %#v", f.model.prompt, want)
		}
		if len(f.convs.outsideTx) > 0 || len(f.convs.deleted) > 0 {
			t.Errorf("writes outside a transaction %v, deleted %v", f.convs.outsideTx, f.convs.deleted)
		}
	})

	t.Run("needs a reaction before anything is created", func(t *testing.T) {
		f := newStart(&fakeModel{})
		_, err := f.uc.Execute(t.Context(), "u1", unreacted.ID, diogenes.ID, f.out)
		if !errors.Is(err, quote.ErrReactionRequired) {
			t.Fatalf("err = %v", err)
		}
		if len(f.convs.created) > 0 || f.out.started != "" || f.model.calls > 0 {
			t.Error("nothing may be created or streamed")
		}
	})

	t.Run("unknown persona", func(t *testing.T) {
		f := newStart(&fakeModel{})
		if _, err := f.uc.Execute(t.Context(), "u1", reacted.ID, "nobody", f.out); !errors.Is(err, kernel.ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	failures := []struct {
		name      string
		model     *fakeModel
		appendErr error
		want      error
	}{
		{"model down", &fakeModel{chunks: []string{"partial"}, err: errors.New("overloaded")}, nil, domain.ErrCharacterUnavailable},
		{"refused", &fakeModel{err: domain.ErrRefused}, nil, domain.ErrRefused},
		{"reply not saved", &fakeModel{chunks: []string{"ok"}}, errors.New("db down"), domain.ErrReplyNotSaved},
	}
	for _, tt := range failures {
		t.Run(tt.name+" removes the empty conversation", func(t *testing.T) {
			f := newStart(tt.model)
			f.convs.appendAssistantErr = tt.appendErr
			_, err := f.uc.Execute(t.Context(), "u1", reacted.ID, diogenes.ID, f.out)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(f.convs.deleted, []string{"conv-q1"}) || len(f.convs.outsideTx) > 0 {
				t.Errorf("deleted %v, writes outside a transaction %v", f.convs.deleted, f.convs.outsideTx)
			}
		})
	}

	t.Run("reports a failed cleanup", func(t *testing.T) {
		f := newStart(&fakeModel{err: errors.New("overloaded")})
		f.convs.deleteErr = errors.New("db down")
		_, err := f.uc.Execute(t.Context(), "u1", reacted.ID, diogenes.ID, f.out)
		var cleanup *CleanupError
		if !errors.Is(err, domain.ErrCharacterUnavailable) || !errors.As(err, &cleanup) {
			t.Fatalf("err = %v, want the reply failure and a CleanupError", err)
		}
	})
}

func newPost(conv domain.Conversation, model *fakeModel) (*PostMessage, *fakeConversations, *fakeStream) {
	convs := &fakeConversations{conv: conv}
	quotes := &fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted, unreacted.ID: unreacted}}
	return NewPostMessage(&portstest.TxManager{}, quotes, convs, model), convs, &fakeStream{}
}

func TestPostMessage(t *testing.T) {
	answered := domain.Conversation{
		ID: "c1", QuoteID: reacted.ID, Persona: diogenes,
		Messages: []domain.Message{{ID: 1, Role: domain.RoleAssistant, Content: "Do you, though?"}},
	}

	t.Run("adds the reader's message and streams the answer", func(t *testing.T) {
		model := &fakeModel{chunks: []string{"Then act on it."}}
		uc, convs, out := newPost(answered, model)
		m, err := uc.Execute(t.Context(), "u1", "c1", "  Sometimes.  ", out)
		if err != nil {
			t.Fatal(err)
		}
		if m.Content != "Then act on it." || out.started != "c1" {
			t.Errorf("reply = %#v, started %q", m, out.started)
		}
		if len(convs.appended) != 2 || convs.appended[0].Role != domain.RoleUser || convs.appended[0].Content != "Sometimes." {
			t.Fatalf("appended %#v", convs.appended)
		}
		wantTurns := []domain.Turn{
			{Role: domain.RoleUser, Content: domain.OpeningMessage(domain.PassageOf(reacted))},
			{Role: domain.RoleAssistant, Content: "Do you, though?"},
			{Role: domain.RoleUser, Content: "Sometimes."},
		}
		if !reflect.DeepEqual(model.prompt.Turns, wantTurns) {
			t.Errorf("turns = %#v", model.prompt.Turns)
		}
		if len(convs.outsideTx) > 0 {
			t.Errorf("writes outside a transaction: %v", convs.outsideTx)
		}
	})

	t.Run("rejects an empty message before loading anything", func(t *testing.T) {
		uc, _, out := newPost(domain.Conversation{}, &fakeModel{})
		if _, err := uc.Execute(t.Context(), "u1", "c1", strings.Repeat(" ", 3), out); !errors.Is(err, domain.ErrMessageLength) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("waits for the character's answer", func(t *testing.T) {
		waiting := answered
		waiting.Messages = append(append([]domain.Message{}, answered.Messages...), domain.Message{ID: 2, Role: domain.RoleUser})
		uc, convs, out := newPost(waiting, &fakeModel{})
		if _, err := uc.Execute(t.Context(), "u1", "c1", "Hello?", out); !errors.Is(err, domain.ErrNotYourTurn) {
			t.Fatalf("err = %v", err)
		}
		if len(convs.appended) > 0 || out.started != "" {
			t.Error("nothing may be stored or streamed")
		}
	})

	t.Run("drops the unanswered message when the reply fails", func(t *testing.T) {
		uc, convs, out := newPost(answered, &fakeModel{err: errors.New("overloaded")})
		if _, err := uc.Execute(t.Context(), "u1", "c1", "Sometimes.", out); !errors.Is(err, domain.ErrCharacterUnavailable) {
			t.Fatalf("err = %v", err)
		}
		if !reflect.DeepEqual(convs.deletedMessages, []int64{convs.appended[0].ID}) || len(convs.outsideTx) > 0 {
			t.Errorf("deleted messages %v, writes outside a transaction %v", convs.deletedMessages, convs.outsideTx)
		}
	})
}

func TestGetThread(t *testing.T) {
	convs := &fakeConversations{list: []domain.Conversation{{ID: "c1"}}}
	uc := NewGetThread(&fakeQuotes{quotes: map[string]quote.Quote{reacted.ID: reacted}}, convs)
	th, err := uc.Execute(t.Context(), "u1", reacted.ID)
	if err != nil || th.Quote.ID != reacted.ID || len(th.Conversations) != 1 {
		t.Errorf("Execute() = %#v, %v", th, err)
	}
	if _, err := uc.Execute(t.Context(), "u1", "missing"); !errors.Is(err, kernel.ErrNotFound) {
		t.Errorf("missing quote: err = %v", err)
	}
}
